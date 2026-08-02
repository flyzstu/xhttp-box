package option

import "github.com/sagernet/sing/common/json/badoption"

type SelectorOutboundOptions struct {
	Outbounds                 []string `json:"outbounds" reference:"outbound"`
	Default                   string   `json:"default,omitempty" reference:"outbound"`
	InterruptExistConnections bool     `json:"interrupt_exist_connections,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds                 []string           `json:"outbounds" reference:"outbound"`
	URL                       string             `json:"url,omitempty"`
	Interval                  badoption.Duration `json:"interval,omitempty"`
	Tolerance                 uint16             `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration `json:"idle_timeout,omitempty"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
}

type WeightedOutboundOptions struct {
	Outbounds    []WeightedOutboundItem      `json:"outbounds"`
	Strategy     string                      `json:"strategy,omitempty" enum:"smooth_wrr"`
	HealthCheck  *WeightedHealthCheckOptions `json:"health_check,omitempty"`
	AllUnhealthy string                      `json:"all_unhealthy,omitempty" enum:"block,fallback"`
	Fallback     string                      `json:"fallback,omitempty" reference:"outbound"`
}

type WeightedOutboundItem struct {
	Tag    string `json:"tag" reference:"outbound"`
	Weight uint32 `json:"weight"`
}

type WeightedHealthCheckOptions struct {
	Enabled          bool               `json:"enabled,omitempty"`
	URL              string             `json:"url,omitempty"`
	Interval         badoption.Duration `json:"interval,omitempty"`
	Timeout          badoption.Duration `json:"timeout,omitempty"`
	FailureThreshold uint32             `json:"failure_threshold,omitempty"`
	SuccessThreshold uint32             `json:"success_threshold,omitempty"`
	Cooldown         badoption.Duration `json:"cooldown,omitempty"`
}
