package dns

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/registry"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

type TransportConstructorFunc[T any] func(ctx context.Context, logger log.ContextLogger, tag string, options T) (adapter.DNSTransport, error)

type TransportRegistry struct {
	*registry.Registry[adapter.DNSTransport]
}

func NewTransportRegistry() *TransportRegistry {
	return &TransportRegistry{registry.New[adapter.DNSTransport]("transport")}
}

func RegisterTransport[Options any](registry *TransportRegistry, transportType string, constructor TransportConstructorFunc[Options]) {
	registry.Register(transportType, func() any {
		return new(Options)
	}, func(ctx context.Context, _ any, logger log.ContextLogger, tag string, rawOptions any) (adapter.DNSTransport, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.DNSTransportRegistry = (*TransportRegistry)(nil)

func (r *TransportRegistry) CreateDNSTransport(ctx context.Context, logger log.ContextLogger, tag string, transportType string, options any) (adapter.DNSTransport, error) {
	return r.Registry.Create(ctx, nil, logger, tag, transportType, options)
}
