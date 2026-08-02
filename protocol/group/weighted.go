package group

import (
	"context"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const (
	weightedStrategySmoothWRR = "smooth_wrr"
	weightedStateHealthy      = "healthy"
	weightedStateQuarantined  = "quarantined"
	weightedStateHalfOpen     = "half-open"

	defaultWeightedHealthInterval         = 30 * time.Second
	defaultWeightedHealthTimeout          = 5 * time.Second
	defaultWeightedHealthFailureThreshold = 3
	defaultWeightedHealthSuccessThreshold = 2
	defaultWeightedHealthCooldown         = 2 * time.Minute
)

func RegisterWeighted(registry *outbound.Registry) {
	outbound.Register[option.WeightedOutboundOptions](registry, C.TypeWeighted, NewWeighted)
}

var (
	_ adapter.OutboundGroup           = (*Weighted)(nil)
	_ adapter.ConnectionHandler       = (*Weighted)(nil)
	_ adapter.PacketConnectionHandler = (*Weighted)(nil)
	_ adapter.InterfaceUpdateListener = (*Weighted)(nil)
)

type Weighted struct {
	outbound.Adapter
	ctx        context.Context
	outbound   adapter.OutboundManager
	connection adapter.ConnectionManager
	logger     log.ContextLogger
	history    *urltest.HistoryStorage

	tags         []string
	members      []*weightedMember
	fallbackTag  string
	fallback     adapter.Outbound
	lastSelected string

	healthEnabled    bool
	healthURL        string
	healthInterval   time.Duration
	healthTimeout    time.Duration
	failureThreshold uint32
	successThreshold uint32
	cooldown         time.Duration

	access    sync.Mutex
	checking  atomic.Bool
	close     chan struct{}
	closeOnce sync.Once
}

type weightedMember struct {
	tag      string
	weight   uint32
	outbound adapter.Outbound

	currentTCP int64
	currentUDP int64

	state                string
	consecutiveFailures  uint32
	consecutiveSuccesses uint32
	quarantinedUntil     time.Time
	lastCheck            time.Time
	lastDelay            uint16
}

type WeightedStatus struct {
	Tag                  string    `json:"tag"`
	Weight               uint32    `json:"weight"`
	State                string    `json:"state"`
	ConsecutiveFailures  uint32    `json:"consecutive_failures"`
	ConsecutiveSuccesses uint32    `json:"consecutive_successes"`
	QuarantinedUntil     time.Time `json:"quarantined_until,omitempty"`
	LastCheck            time.Time `json:"last_check,omitempty"`
	LastDelay            uint16    `json:"last_delay,omitempty"`
}

func NewWeighted(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WeightedOutboundOptions) (adapter.Outbound, error) {
	if len(options.Outbounds) == 0 {
		return nil, E.New("missing weighted outbounds")
	}
	if options.Strategy != "" && options.Strategy != weightedStrategySmoothWRR {
		return nil, E.New("unsupported weighted strategy: ", options.Strategy)
	}
	allUnhealthy := options.AllUnhealthy
	if allUnhealthy == "" {
		if options.Fallback != "" {
			allUnhealthy = "fallback"
		} else {
			allUnhealthy = "block"
		}
	}
	if allUnhealthy != "block" && allUnhealthy != "fallback" {
		return nil, E.New("unknown all_unhealthy behavior: ", allUnhealthy)
	}
	if allUnhealthy == "fallback" && options.Fallback == "" {
		return nil, E.New("fallback is required when all_unhealthy is fallback")
	}
	if allUnhealthy == "block" && options.Fallback != "" {
		return nil, E.New("fallback requires all_unhealthy to be fallback")
	}

	tags := make([]string, 0, len(options.Outbounds))
	dependencies := make([]string, 0, len(options.Outbounds)+1)
	members := make([]*weightedMember, 0, len(options.Outbounds))
	seen := make(map[string]bool, len(options.Outbounds))
	for index, item := range options.Outbounds {
		if item.Tag == "" {
			return nil, E.New("missing weighted outbound tag at index ", index)
		}
		if item.Weight == 0 {
			return nil, E.New("weighted outbound ", item.Tag, " has zero weight")
		}
		if seen[item.Tag] {
			return nil, E.New("duplicate weighted outbound: ", item.Tag)
		}
		seen[item.Tag] = true
		tags = append(tags, item.Tag)
		dependencies = append(dependencies, item.Tag)
		members = append(members, &weightedMember{
			tag:    item.Tag,
			weight: item.Weight,
			state:  weightedStateHealthy,
		})
	}
	if options.Fallback != "" {
		if seen[options.Fallback] {
			return nil, E.New("fallback must not also be a weighted outbound")
		}
		dependencies = append(dependencies, options.Fallback)
	}

	weighted := &Weighted{
		Adapter:      outbound.NewAdapter(C.TypeWeighted, tag, []string{N.NetworkTCP, N.NetworkUDP}, dependencies),
		ctx:          ctx,
		outbound:     service.FromContext[adapter.OutboundManager](ctx),
		connection:   service.FromContext[adapter.ConnectionManager](ctx),
		logger:       logger,
		history:      service.PtrFromContext[urltest.HistoryStorage](ctx),
		tags:         tags,
		members:      members,
		fallbackTag:  options.Fallback,
		lastSelected: tags[0],
		close:        make(chan struct{}),
	}
	if options.HealthCheck != nil && options.HealthCheck.Enabled {
		health := options.HealthCheck
		weighted.healthEnabled = true
		weighted.healthURL = health.URL
		weighted.healthInterval = time.Duration(health.Interval)
		weighted.healthTimeout = time.Duration(health.Timeout)
		weighted.failureThreshold = health.FailureThreshold
		weighted.successThreshold = health.SuccessThreshold
		weighted.cooldown = time.Duration(health.Cooldown)
		if weighted.healthInterval == 0 {
			weighted.healthInterval = defaultWeightedHealthInterval
		}
		if weighted.healthTimeout == 0 {
			weighted.healthTimeout = defaultWeightedHealthTimeout
		}
		if weighted.failureThreshold == 0 {
			weighted.failureThreshold = defaultWeightedHealthFailureThreshold
		}
		if weighted.successThreshold == 0 {
			weighted.successThreshold = defaultWeightedHealthSuccessThreshold
		}
		if weighted.cooldown == 0 {
			weighted.cooldown = defaultWeightedHealthCooldown
		}
		if weighted.healthInterval < 0 || weighted.healthTimeout < 0 || weighted.cooldown < 0 {
			return nil, E.New("weighted health durations must be positive")
		}
	}
	return weighted, nil
}

func (w *Weighted) Start() error {
	for _, member := range w.members {
		detour, loaded := w.outbound.Outbound(member.tag)
		if !loaded {
			return E.New("weighted outbound not found: ", member.tag)
		}
		member.outbound = detour
	}
	if w.fallbackTag != "" {
		fallback, loaded := w.outbound.Outbound(w.fallbackTag)
		if !loaded {
			return E.New("weighted fallback outbound not found: ", w.fallbackTag)
		}
		w.fallback = fallback
	}
	return nil
}

func (w *Weighted) PostStart() error {
	if !w.healthEnabled {
		return nil
	}
	go w.healthLoop()
	return nil
}

func (w *Weighted) Close() error {
	w.closeOnce.Do(func() {
		close(w.close)
	})
	return nil
}

func (w *Weighted) InterfaceUpdated() {
	if w.healthEnabled {
		go w.checkOutbounds()
	}
}

func (w *Weighted) Now() string {
	w.access.Lock()
	defer w.access.Unlock()
	return w.lastSelected
}

func (w *Weighted) All() []string {
	return append([]string(nil), w.tags...)
}

func (w *Weighted) Status() []WeightedStatus {
	w.access.Lock()
	defer w.access.Unlock()
	status := make([]WeightedStatus, 0, len(w.members))
	for _, member := range w.members {
		status = append(status, WeightedStatus{
			Tag:                  member.tag,
			Weight:               member.weight,
			State:                member.state,
			ConsecutiveFailures:  member.consecutiveFailures,
			ConsecutiveSuccesses: member.consecutiveSuccesses,
			QuarantinedUntil:     member.quarantinedUntil,
			LastCheck:            member.lastCheck,
			LastDelay:            member.lastDelay,
		})
	}
	return status
}

func (w *Weighted) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	detour, member := w.selectOutbound(network)
	if detour == nil {
		return nil, syscall.EPERM
	}
	conn, err := detour.DialContext(ctx, network, destination)
	if member != nil {
		w.recordPassiveResult(member, err)
	}
	return conn, err
}

func (w *Weighted) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	detour, member := w.selectOutbound(N.NetworkUDP)
	if detour == nil {
		return nil, syscall.EPERM
	}
	conn, err := detour.ListenPacket(ctx, destination)
	if member != nil {
		w.recordPassiveResult(member, err)
	}
	return conn, err
}

func (w *Weighted) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	w.connection.NewConnection(ctx, w, conn, metadata, onClose)
}

func (w *Weighted) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	w.connection.NewPacketConnection(ctx, w, conn, metadata, onClose)
}

func (w *Weighted) selectOutbound(network string) (adapter.Outbound, *weightedMember) {
	w.access.Lock()
	defer w.access.Unlock()
	var selected *weightedMember
	var total int64
	for _, member := range w.members {
		if member.state != weightedStateHealthy || !containsNetwork(member.outbound.Network(), network) {
			continue
		}
		weight := int64(member.weight)
		total += weight
		if network == N.NetworkUDP {
			member.currentUDP += weight
			if selected == nil || member.currentUDP > selected.currentUDP {
				selected = member
			}
		} else {
			member.currentTCP += weight
			if selected == nil || member.currentTCP > selected.currentTCP {
				selected = member
			}
		}
	}
	if selected == nil {
		if w.fallback != nil && containsNetwork(w.fallback.Network(), network) {
			w.lastSelected = w.fallback.Tag()
			return w.fallback, nil
		}
		w.lastSelected = ""
		return nil, nil
	}
	if network == N.NetworkUDP {
		selected.currentUDP -= total
	} else {
		selected.currentTCP -= total
	}
	w.lastSelected = selected.tag
	return selected.outbound, selected
}

func containsNetwork(networks []string, network string) bool {
	return slices.Contains(networks, network)
}

func (w *Weighted) recordPassiveResult(member *weightedMember, err error) {
	if !w.healthEnabled {
		return
	}
	w.access.Lock()
	if member.state != weightedStateHealthy {
		w.access.Unlock()
		return
	}
	if err == nil {
		member.consecutiveFailures = 0
		w.access.Unlock()
		return
	}
	w.recordFailureLocked(member, time.Now())
	quarantined := member.state == weightedStateQuarantined
	w.access.Unlock()
	if quarantined && w.history != nil {
		w.history.DeleteURLTestHistory(RealTag(member.outbound))
	}
}

func (w *Weighted) recordFailureLocked(member *weightedMember, now time.Time) {
	member.consecutiveFailures++
	member.consecutiveSuccesses = 0
	if member.state == weightedStateHalfOpen || member.consecutiveFailures >= w.failureThreshold {
		member.state = weightedStateQuarantined
		member.quarantinedUntil = now.Add(w.cooldown)
		member.currentTCP = 0
		member.currentUDP = 0
		w.logger.Warn("weighted outbound ", member.tag, " quarantined until ", member.quarantinedUntil.Format(time.RFC3339))
	}
}

func (w *Weighted) healthLoop() {
	w.checkOutbounds()
	ticker := time.NewTicker(w.healthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.checkOutbounds()
		case <-w.close:
			return
		}
	}
}

func (w *Weighted) checkOutbounds() {
	if w.checking.Swap(true) {
		return
	}
	defer w.checking.Store(false)
	var waitGroup sync.WaitGroup
	for _, member := range w.members {
		if !w.prepareProbe(member, time.Now()) {
			continue
		}
		waitGroup.Add(1)
		go func(member *weightedMember) {
			defer waitGroup.Done()
			checkContext, cancel := context.WithTimeout(w.ctx, w.healthTimeout)
			defer cancel()
			delay, err := urltest.URLTest(checkContext, w.healthURL, member.outbound)
			w.recordProbeResult(member, delay, err)
		}(member)
	}
	waitGroup.Wait()
}

func (w *Weighted) prepareProbe(member *weightedMember, now time.Time) bool {
	w.access.Lock()
	defer w.access.Unlock()
	if member.state == weightedStateQuarantined {
		if now.Before(member.quarantinedUntil) {
			return false
		}
		member.state = weightedStateHalfOpen
		member.consecutiveSuccesses = 0
		member.consecutiveFailures = 0
	}
	return true
}

func (w *Weighted) recordProbeResult(member *weightedMember, delay uint16, err error) {
	w.access.Lock()
	now := time.Now()
	member.lastCheck = now
	if err != nil {
		member.lastDelay = 0
		w.recordFailureLocked(member, now)
		w.access.Unlock()
		if w.history != nil {
			w.history.DeleteURLTestHistory(RealTag(member.outbound))
		}
		return
	}
	member.lastDelay = delay
	member.consecutiveFailures = 0
	if member.state == weightedStateHalfOpen {
		member.consecutiveSuccesses++
		if member.consecutiveSuccesses >= w.successThreshold {
			member.state = weightedStateHealthy
			member.consecutiveSuccesses = 0
			member.quarantinedUntil = time.Time{}
			w.logger.Info("weighted outbound ", member.tag, " recovered")
		}
	} else {
		member.consecutiveSuccesses = 0
	}
	w.access.Unlock()
	if w.history != nil {
		w.history.StoreURLTestHistory(RealTag(member.outbound), &adapter.URLTestHistory{
			Time:  now,
			Delay: delay,
		})
	}
}
