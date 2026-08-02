package dialer

import (
	"context"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"

	"github.com/stretchr/testify/require"
)

func TestResolveDialerOptionsForInbound(t *testing.T) {
	defaultOptions := adapter.DNSQueryOptions{Strategy: C.DomainStrategyPreferIPv4}
	inboundOptions := adapter.DNSQueryOptions{Strategy: C.DomainStrategyIPv6Only}
	dialer := &resolveDialer{
		queryOptions: defaultOptions,
		inbound: map[string]adapter.DomainResolverOptions{
			"vless-ipv6-in": {
				Server:       "dns-ipv6",
				QueryOptions: inboundOptions,
			},
		},
	}

	require.Equal(t, defaultOptions, dialer.optionsForContext(context.Background()))
	require.Equal(t, defaultOptions, dialer.optionsForContext(adapter.WithContext(context.Background(), &adapter.InboundContext{Inbound: "other-in"})))
	require.Equal(t, inboundOptions, dialer.optionsForContext(adapter.WithContext(context.Background(), &adapter.InboundContext{Inbound: "vless-ipv6-in"})))
}
