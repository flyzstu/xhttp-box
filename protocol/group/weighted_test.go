package group

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	adapterOutbound "github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/require"
)

type weightedTestOutbound struct {
	adapterOutbound.Adapter
}

func newWeightedTestOutbound(tag string, networks ...string) *weightedTestOutbound {
	return &weightedTestOutbound{
		Adapter: adapterOutbound.NewAdapter("test", tag, networks, nil),
	}
}

func (*weightedTestOutbound) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return nil, nil
}

func (*weightedTestOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

func newWeightedForTest(weights ...uint32) *Weighted {
	members := make([]*weightedMember, 0, len(weights))
	tags := make([]string, 0, len(weights))
	for index, weight := range weights {
		tag := string(rune('a' + index))
		tags = append(tags, tag)
		members = append(members, &weightedMember{
			tag:      tag,
			weight:   weight,
			outbound: newWeightedTestOutbound(tag, N.NetworkTCP, N.NetworkUDP),
			state:    weightedStateHealthy,
		})
	}
	return &Weighted{
		tags:             tags,
		members:          members,
		logger:           log.NewNOPFactory().NewLogger("weighted-test"),
		healthEnabled:    true,
		failureThreshold: 2,
		successThreshold: 2,
		cooldown:         time.Minute,
	}
}

func TestWeightedSmoothWRRDistribution(t *testing.T) {
	weighted := newWeightedForTest(5, 3, 2)
	counts := make(map[string]int)
	for range 100 {
		selected, member := weighted.selectOutbound(N.NetworkTCP)
		require.NotNil(t, selected)
		require.NotNil(t, member)
		counts[member.tag]++
	}
	require.Equal(t, map[string]int{"a": 50, "b": 30, "c": 20}, counts)

	udpCounts := make(map[string]int)
	for range 100 {
		_, member := weighted.selectOutbound(N.NetworkUDP)
		require.NotNil(t, member)
		udpCounts[member.tag]++
	}
	require.Equal(t, map[string]int{"a": 50, "b": 30, "c": 20}, udpCounts)
}

func TestWeightedConcurrentDistribution(t *testing.T) {
	weighted := newWeightedForTest(3, 1)
	counts := make(map[string]int)
	var countsAccess sync.Mutex
	var waitGroup sync.WaitGroup
	for range 32 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range 1000 {
				_, member := weighted.selectOutbound(N.NetworkTCP)
				countsAccess.Lock()
				counts[member.tag]++
				countsAccess.Unlock()
			}
		}()
	}
	waitGroup.Wait()
	require.Equal(t, map[string]int{"a": 24000, "b": 8000}, counts)
}

func TestWeightedQuarantineAndRecovery(t *testing.T) {
	weighted := newWeightedForTest(1, 1)
	failedMember := weighted.members[0]
	weighted.recordPassiveResult(failedMember, errors.New("dial failed"))
	require.Equal(t, weightedStateHealthy, failedMember.state)
	weighted.recordPassiveResult(failedMember, errors.New("dial failed"))
	require.Equal(t, weightedStateQuarantined, failedMember.state)

	for range 10 {
		_, member := weighted.selectOutbound(N.NetworkTCP)
		require.Equal(t, "b", member.tag)
	}

	weighted.access.Lock()
	failedMember.quarantinedUntil = time.Now().Add(-time.Second)
	weighted.access.Unlock()
	require.True(t, weighted.prepareProbe(failedMember, time.Now()))
	require.Equal(t, weightedStateHalfOpen, failedMember.state)
	weighted.recordProbeResult(failedMember, 20, nil)
	require.Equal(t, weightedStateHalfOpen, failedMember.state)
	weighted.recordProbeResult(failedMember, 20, nil)
	require.Equal(t, weightedStateHealthy, failedMember.state)
}

func TestWeightedAllUnhealthyBehavior(t *testing.T) {
	weighted := newWeightedForTest(1)
	weighted.members[0].state = weightedStateQuarantined
	selected, member := weighted.selectOutbound(N.NetworkTCP)
	require.Nil(t, selected)
	require.Nil(t, member)

	fallback := newWeightedTestOutbound("fallback", N.NetworkTCP, N.NetworkUDP)
	weighted.fallback = fallback
	selected, member = weighted.selectOutbound(N.NetworkTCP)
	require.Equal(t, adapter.Outbound(fallback), selected)
	require.Nil(t, member)
}

func TestNewWeightedValidation(t *testing.T) {
	testCases := []struct {
		name    string
		options option.WeightedOutboundOptions
	}{
		{name: "missing outbounds"},
		{
			name: "zero weight",
			options: option.WeightedOutboundOptions{
				Outbounds: []option.WeightedOutboundItem{{Tag: "a"}},
			},
		},
		{
			name: "duplicate tag",
			options: option.WeightedOutboundOptions{
				Outbounds: []option.WeightedOutboundItem{{Tag: "a", Weight: 1}, {Tag: "a", Weight: 1}},
			},
		},
		{
			name: "missing fallback",
			options: option.WeightedOutboundOptions{
				Outbounds:    []option.WeightedOutboundItem{{Tag: "a", Weight: 1}},
				AllUnhealthy: "fallback",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewWeighted(context.Background(), nil, log.NewNOPFactory().NewLogger("weighted-test"), "weighted", testCase.options)
			require.Error(t, err)
		})
	}
}
