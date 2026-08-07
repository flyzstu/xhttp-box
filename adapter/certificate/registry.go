package certificate

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/registry"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

type ConstructorFunc[T any] func(ctx context.Context, logger log.ContextLogger, tag string, options T) (adapter.CertificateProviderService, error)

type Registry struct {
	*registry.Registry[adapter.CertificateProviderService]
}

func NewRegistry() *Registry {
	return &Registry{registry.New[adapter.CertificateProviderService]("certificate provider")}
}

func Register[Options any](registry *Registry, providerType string, constructor ConstructorFunc[Options]) {
	registry.Register(providerType, func() any {
		return new(Options)
	}, func(ctx context.Context, _ any, logger log.ContextLogger, tag string, rawOptions any) (adapter.CertificateProviderService, error) {
		var options *Options
		if rawOptions != nil {
			options = rawOptions.(*Options)
		}
		return constructor(ctx, logger, tag, common.PtrValueOrDefault(options))
	})
}

var _ adapter.CertificateProviderRegistry = (*Registry)(nil)

func (m *Registry) Create(ctx context.Context, logger log.ContextLogger, tag string, providerType string, options any) (adapter.CertificateProviderService, error) {
	return m.Registry.Create(ctx, nil, logger, tag, providerType, options)
}
