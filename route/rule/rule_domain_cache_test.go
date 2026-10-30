package rule

import (
	"context"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

func TestDefaultDNSRule_RequireDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := log.NewNOPFactory().NewLogger("dns")

	domainRule, err := NewDefaultDNSRule(ctx, logger, option.DefaultDNSRule{
		RawDefaultDNSRule: option.RawDefaultDNSRule{
			Domain: []string{"example.com"},
		},
	}, true)
	require.NoError(t, err)
	require.True(t, domainRule.RequireDomain())

	addressRule, err := NewDefaultDNSRule(ctx, logger, option.DefaultDNSRule{
		RawDefaultDNSRule: option.RawDefaultDNSRule{
			IPCIDR: []string{"8.8.8.8/32"},
		},
	}, true)
	require.NoError(t, err)
	require.False(t, addressRule.RequireDomain())

	invertRule, err := NewDefaultDNSRule(ctx, logger, option.DefaultDNSRule{
		RawDefaultDNSRule: option.RawDefaultDNSRule{
			Domain: []string{"example.com"},
			Invert: true,
		},
	}, true)
	require.NoError(t, err)
	require.False(t, invertRule.RequireDomain())
}

func TestInboundContext_DomainHost(t *testing.T) {
	t.Parallel()

	metadata := &adapter.InboundContext{
		Domain: "EXAMPLE.com",
	}
	require.Equal(t, "example.com", metadata.DomainHost())
	require.Equal(t, "example.com", metadata.DomainHost())

	metadata.Domain = ""
	metadata.Destination = M.ParseSocksaddr("www.Example.org:443")
	require.Equal(t, "www.example.org", metadata.DomainHost())
	require.Equal(t, "www.example.org", metadata.DomainHost())

	metadata.Domain = "OTHER.net"
	require.Equal(t, "other.net", metadata.DomainHost())
}

func TestInboundContext_DomainHost_Empty(t *testing.T) {
	t.Parallel()

	metadata := &adapter.InboundContext{}
	require.Equal(t, "", metadata.DomainHost())

	metadata.Destination = M.ParseSocksaddr("1.2.3.4:53")
	require.Equal(t, "", metadata.DomainHost())
}
