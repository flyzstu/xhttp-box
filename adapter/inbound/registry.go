package inbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/registry"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

type ConstructorFunc[T any] func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options T) (adapter.Inbound, error)

type Registry struct {
	*registry.Registry[adapter.Inbound]
}

func NewRegistry() *Registry {
	return &Registry{registry.New[adapter.Inbound]("outbound")}
}

func Register[Options any](registry *Registry, outboundType string, constructor ConstructorFunc[Options]) {
	registry.Register(outboundType, func() any {
		return new(Options)
	}, func(ctx context.Context, router any, logger log.ContextLogger, tag string, rawOptions any) (adapter.Inbound, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, router.(adapter.Router), logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.InboundRegistry = (*Registry)(nil)

func (m *Registry) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, inboundType string, options any) (adapter.Inbound, error) {
	return m.Registry.Create(ctx, router, logger, tag, inboundType, options)
}
