package v2rayxhttp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2/hpack"
)

func TestResolvedMode(t *testing.T) {
	require.Equal(t, "packet-up", resolvedMode("auto", false, false))
	require.Equal(t, "stream-one", resolvedMode("auto", true, false))
	require.Equal(t, "stream-up", resolvedMode("auto", true, true))
	require.Equal(t, "stream-up", resolvedMode("stream-up", true, false))
}

func TestDownloadSettingsJSON(t *testing.T) {
	options := option.V2RayXHTTPOptions{
		Path: "/upload",
		DownloadSettings: &option.V2RayXHTTPDownloadSettings{
			ServerOptions:               option.ServerOptions{Server: "download.example", ServerPort: 443},
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "download.example"}},
			V2RayXHTTPOptions:           option.V2RayXHTTPOptions{Path: "/download", Mode: "packet-up"},
		},
	}
	encoded, err := json.Marshal(options)
	require.NoError(t, err)
	var decoded option.V2RayXHTTPOptions
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.NotNil(t, decoded.DownloadSettings)
	require.Equal(t, "download.example", decoded.DownloadSettings.Server)
	require.Equal(t, uint16(443), decoded.DownloadSettings.ServerPort)
	require.Equal(t, "/download", decoded.DownloadSettings.Path)
	require.True(t, decoded.DownloadSettings.TLS.Enabled)
}

func TestPacketPayloadChunking(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 600)
	for _, placement := range []string{placementHeader, placementCookie} {
		t.Run(placement, func(t *testing.T) {
			config, err := newConfig(option.V2RayXHTTPOptions{
				UplinkDataPlacement: placement,
				UplinkChunkSize:     option.V2RayXHTTPRange{From: 65, To: 65},
			})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, "http://example.com/xhttp/", nil)
			config.fillPacketRequest(request, "session", 0, payload)
			decoded, err := config.extractPacketPayload(request, 1000000)
			require.NoError(t, err)
			require.Equal(t, payload, decoded)
			for index := 0; ; index++ {
				var chunk string
				if placement == placementHeader {
					chunk = request.Header.Get(config.dataKey + "-" + strconv.Itoa(index))
				} else if cookie, _ := request.Cookie(config.dataKey + "_" + strconv.Itoa(index)); cookie != nil {
					chunk = cookie.Value
				}
				if chunk == "" {
					break
				}
				require.LessOrEqual(t, len(chunk), 65)
			}
		})
	}
}

func TestQueryHelpers(t *testing.T) {
	require.Equal(t, "a=1&x_padding=2", appendQuery("a=1", "x_padding", "2"))
	require.Equal(t, "a=1&x_padding=2", appendQuery("a=1&x_padding=9", "x_padding", "2"))
	require.Equal(t, "x_p=9", appendQuery("x%5Fp=1", "x_p", "9"))
	require.Equal(t, "abc", queryGet("x%5Fsession=abc", "x_session"))
	require.Equal(t, "abc", queryGet("x_session=abc&x_session=def", "x_session"))
	require.Empty(t, queryGet("x_session=%zz", "x_session"))
	// url.ParseQuery rejects pairs containing semicolons, matching the original
	// URL.Query().Get behaviour.
	require.Empty(t, queryGet("x_session=abc;y=1", "x_session"))
	require.Empty(t, queryGet("x_session=abc", "x_session2"))
}

func TestPacketPayloadAuto(t *testing.T) {
	config, err := newConfig(option.V2RayXHTTPOptions{UplinkDataPlacement: placementAuto})
	require.NoError(t, err)
	payload := bytes.Repeat([]byte("y"), 4096)

	t.Run("body", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "http://example.com/xhttp/", bytes.NewReader(payload))
		decoded, err := config.extractPacketPayload(request, 1<<20)
		require.NoError(t, err)
		require.Equal(t, payload, decoded)
	})
	t.Run("header-cookie-body", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "http://example.com/xhttp/", bytes.NewReader(payload))
		request.Header.Set(config.dataKey+"-0", base64.RawURLEncoding.EncodeToString([]byte("head")))
		request.AddCookie(&http.Cookie{Name: config.dataKey + "_0", Value: base64.RawURLEncoding.EncodeToString([]byte("cookie"))})
		decoded, err := config.extractPacketPayload(request, 1<<20)
		require.NoError(t, err)
		require.Equal(t, append([]byte("head"), append([]byte("cookie"), payload...)...), decoded)
	})
}

func TestResponsePadding(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		config, err := newConfig(option.V2RayXHTTPOptions{
			XPaddingBytes: option.V2RayXHTTPRange{From: 12, To: 12},
		})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		config.applyResponsePadding(recorder)
		require.Len(t, recorder.Header().Get("X-Padding"), 12)
	})
	t.Run("obfs-header", func(t *testing.T) {
		config, err := newConfig(option.V2RayXHTTPOptions{
			XPaddingBytes:     option.V2RayXHTTPRange{From: 12, To: 12},
			XPaddingObfsMode:  true,
			XPaddingPlacement: placementHeader,
			XPaddingHeader:    "X-Test-Padding",
			XPaddingMethod:    "tokenish",
		})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		config.applyResponsePadding(recorder)
		value := recorder.Header().Get("X-Test-Padding")
		require.NotEmpty(t, value)
		require.InDelta(t, 12, hpack.HuffmanEncodeLength(value), 2)
	})
}
