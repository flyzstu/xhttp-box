package include

import (
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/protocol/masque"
)

func registerMASQUEEndpoint(registry *endpoint.Registry) {
	masque.RegisterEndpoint(registry)
}
