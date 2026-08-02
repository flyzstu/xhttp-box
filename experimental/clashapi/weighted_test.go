package clashapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/stretchr/testify/require"
)

func TestWeightedProxyInfoIsClashLoadBalance(t *testing.T) {
	detour, err := group.NewWeighted(
		context.Background(),
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
