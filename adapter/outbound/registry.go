package outbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/registry"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

type ConstructorFunc[T any] func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options T) (adapter.Outbound, error)

type Registry struct {
	*registry.Registry[adapter.Outbound]
}

func NewRegistry() *Registry {
	return &Registry{registry.New[adapter.Outbound]("outbound")}
}

func Register[Options any](registry *Registry, outboundType string, constructor ConstructorFunc[Options]) {
	registry.Register(outboundType, func() any {
		return new(Options)
	}, func(ctx context.Context, router any, logger log.ContextLogger, tag string, rawOptions any) (adapter.Outbound, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, router.(adapter.Router), logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.OutboundRegistry = (*Registry)(nil)

func (m *Registry) CreateOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) (adapter.Outbound, error) {
	return m.Registry.Create(ctx, router, logger, tag, outboundType, options)
}
