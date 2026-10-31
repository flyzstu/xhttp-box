package clashapi

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

type weightedTestOutbound struct {
	tag string
}

func (o *weightedTestOutbound) Type() string           { return "direct" }
func (o *weightedTestOutbound) Tag() string            { return o.tag }
func (o *weightedTestOutbound) Network() []string      { return []string{N.NetworkTCP, N.NetworkUDP} }
func (o *weightedTestOutbound) Dependencies() []string { return nil }

func (o *weightedTestOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, io.EOF
}

func (o *weightedTestOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, io.EOF
}

type weightedTestOutboundManager struct {
	outbounds map[string]adapter.Outbound
}

func (m *weightedTestOutboundManager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	return nil
}

func (m *weightedTestOutboundManager) Outbounds() []adapter.Outbound {
	var result []adapter.Outbound
	for _, outbound := range m.outbounds {
		result = append(result, outbound)
	}
	return result
}

func (m *weightedTestOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := m.outbounds[tag]
	return outbound, loaded
}

func (m *weightedTestOutboundManager) Default() adapter.Outbound { return nil }

func (m *weightedTestOutboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) error {
	return io.ErrClosedPipe
}

func TestWeightedProxyInfoIsClashLoadBalance(t *testing.T) {
	manager := &weightedTestOutboundManager{
		outbounds: map[string]adapter.Outbound{
			"proxy-a": &weightedTestOutbound{tag: "proxy-a"},
			"proxy-b": &weightedTestOutbound{tag: "proxy-b"},
		},
	}
	detour, err := group.NewWeighted(
		service.ContextWith[adapter.OutboundManager](context.Background(), manager),
		nil,
		log.NewNOPFactory().NewLogger("weighted-test"),
		"proxy",
		option.WeightedOutboundOptions{
			Outbounds: []option.WeightedOutboundItem{
				{Tag: "proxy-a", Weight: 70},
				{Tag: "proxy-b", Weight: 30},
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, detour.(*group.Weighted).Start())

	server := &Server{urlTestHistory: urltest.NewHistoryStorage()}
	response, err := proxyInfo(server, detour).MarshalJSON()
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(response, &payload))
	require.Equal(t, "LoadBalance", payload["type"])
	require.Equal(t, "proxy", payload["name"])
	require.Equal(t, "proxy-a", payload["now"])
	require.Equal(t, []any{"proxy-a", "proxy-b"}, payload["all"])
	require.Len(t, payload["weighted_status"], 2)
}
