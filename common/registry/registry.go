package registry

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

// Registry stores option type constructors and item constructors keyed by type name.
//
// Constructors are stored with a generic router argument that is passed through
// untyped. Registries without a router pass nil.
type Registry[T any] struct {
	access       sync.Mutex
	typeName     string
	optionsType  map[string]func() any
	constructors map[string]func(ctx context.Context, router any, logger log.ContextLogger, tag string, options any) (T, error)
}

func New[T any](typeName string) *Registry[T] {
	return &Registry[T]{
		typeName:     typeName,
		optionsType:  make(map[string]func() any),
		constructors: make(map[string]func(ctx context.Context, router any, logger log.ContextLogger, tag string, options any) (T, error)),
	}
}

func (r *Registry[T]) OptionTypes() []string {
	r.access.Lock()
	defer r.access.Unlock()
	return slices.Sorted(maps.Keys(r.optionsType))
}

func (r *Registry[T]) CreateOptions(name string) (any, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	optionsConstructor, loaded := r.optionsType[name]
	if !loaded {
		return nil, false
	}
	return optionsConstructor(), true
}

func (r *Registry[T]) Register(name string, optionsConstructor func() any, constructor func(ctx context.Context, router any, logger log.ContextLogger, tag string, options any) (T, error)) {
	r.access.Lock()
	defer r.access.Unlock()
	r.optionsType[name] = optionsConstructor
	r.constructors[name] = constructor
}

// Create invokes the constructor registered under name.
// router is passed through to the constructor and may be nil for registries
// without a router.
func (r *Registry[T]) Create(ctx context.Context, router any, logger log.ContextLogger, tag string, name string, options any) (T, error) {
	r.access.Lock()
	defer r.access.Unlock()
	constructor, loaded := r.constructors[name]
	if !loaded {
		return *new(T), E.New(r.typeName + " type not found: " + name)
	}
	return constructor(ctx, router, logger, tag, options)
}
