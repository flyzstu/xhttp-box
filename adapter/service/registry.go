package service

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/registry"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

type ConstructorFunc[T any] func(ctx context.Context, logger log.ContextLogger, tag string, options T) (adapter.Service, error)

type Registry struct {
	*registry.Registry[adapter.Service]
}

func NewRegistry() *Registry {
	return &Registry{registry.New[adapter.Service]("outbound")}
}

func Register[Options any](registry *Registry, outboundType string, constructor ConstructorFunc[Options]) {
	registry.Register(outboundType, func() any {
		return new(Options)
	}, func(ctx context.Context, _ any, logger log.ContextLogger, tag string, rawOptions any) (adapter.Service, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.ServiceRegistry = (*Registry)(nil)

func (m *Registry) Create(ctx context.Context, logger log.ContextLogger, tag string, serviceType string, options any) (adapter.Service, error) {
	return m.Registry.Create(ctx, nil, logger, tag, serviceType, options)
}
