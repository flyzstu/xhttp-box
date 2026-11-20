package v2rayxhttp

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
)

// benchmarkServer starts an in-process XHTTP server over plain TCP and returns
// its address. It exercises the real HTTP handler path without TLS or a proxy
// protocol, isolating transport framing costs.
func benchmarkServer(b *testing.B, mode string) (M.Socksaddr, func()) {
	b.Helper()
	options := option.V2RayXHTTPOptions{Path: "/xhttp", Mode: mode, SCMinPostsIntervalMS: option.V2RayXHTTPRange{From: 1, To: 1}}
	server, err := NewServer(context.Background(), logger.NOP(), options, nil, echoHandler{})
	if err != nil {
		b.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	go server.Serve(listener)
	return M.SocksaddrFromNet(listener.Addr()), func() {
		_ = listener.Close()
		_ = server.Close()
	}
}

func benchmarkDial(b *testing.B, mode string) (net.Conn, func()) {
	b.Helper()
	addr, closeServer := benchmarkServer(b, mode)
	dialer, err := dialer.NewDefault(context.Background(), option.DialerOptions{})
	if err != nil {
		b.Fatal(err)
	}
	options := option.V2RayXHTTPOptions{Path: "/xhttp", Mode: mode, SCMinPostsIntervalMS: option.V2RayXHTTPRange{From: 1, To: 1}}
	client, err := NewClient(context.Background(), dialer, addr, options, nil)
	if err != nil {
		b.Fatal(err)
	}
	conn, err := client.DialContext(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	return conn, func() {
		_ = conn.Close()
		_ = client.Close()
		closeServer()
	}
}

// BenchmarkXHTTPEcho measures a 64 KiB request/response round trip over one
// logical XHTTP connection. Reported MB/s counts both directions.
func BenchmarkXHTTPEcho(b *testing.B) {
	for _, mode := range []string{"packet-up", "stream-up", "stream-one"} {
		b.Run(mode, func(b *testing.B) {
			conn, closeConn := benchmarkDial(b, mode)
			defer closeConn()
			payload := make([]byte, 64*1024)
			for index := range payload {
				payload[index] = byte(index)
			}
			response := make([]byte, len(payload))
			b.SetBytes(int64(2 * len(payload)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := conn.Write(payload); err != nil {
					b.Fatal(err)
				}
				if _, err := io.ReadFull(conn, response); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkXHTTPSmallWrite measures the per-write cost of the packet-up
// uplink, which creates one HTTP request per write burst.
func BenchmarkXHTTPSmallWrite(b *testing.B) {
	conn, closeConn := benchmarkDial(b, "packet-up")
	defer closeConn()
	payload := make([]byte, 1024)
	response := make([]byte, len(payload))
	b.SetBytes(int64(2 * len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := conn.Write(payload); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(conn, response); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkConfig(b *testing.B, options option.V2RayXHTTPOptions) *config {
	b.Helper()
	config, err := newConfig(options)
	if err != nil {
		b.Fatal(err)
	}
	return config
}

// BenchmarkXHTTPRequestBuild isolates client-side request construction for a
// packet-up POST with the default body payload placement.
func BenchmarkXHTTPRequestBuild(b *testing.B) {
	config := benchmarkConfig(b, option.V2RayXHTTPOptions{Path: "/xhttp", Mode: "packet-up"})
	payload := make([]byte, 64*1024)
	request, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:1/xhttp/", nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		request.URL.Path = "/xhttp/"
		config.fillPacketRequest(request, "0123456789abcdef", uint64(index), payload)
	}
}

// BenchmarkXHTTPPaddingValue isolates the padding generator, including the
// tokenish Huffman-length search.
func BenchmarkXHTTPPaddingValue(b *testing.B) {
	for _, method := range []string{"repeat-x", "tokenish"} {
		b.Run(method, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = paddingValue(method, 500)
			}
		})
	}
}

// BenchmarkXHTTPApplyPadding isolates request padding placement.
func BenchmarkXHTTPApplyPadding(b *testing.B) {
	for _, options := range []option.V2RayXHTTPOptions{
		{Path: "/xhttp", XPaddingBytes: option.V2RayXHTTPRange{From: 100, To: 1000}},
		{Path: "/xhttp", XPaddingBytes: option.V2RayXHTTPRange{From: 100, To: 1000}, XPaddingObfsMode: true, XPaddingPlacement: "header", XPaddingHeader: "X-Padding", XPaddingMethod: "tokenish"},
	} {
		config := benchmarkConfig(b, options)
		request, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:1/xhttp/session", nil)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(config.paddingMethod+"/"+config.paddingPlacement, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				config.applyPadding(request)
			}
		})
	}
}

// BenchmarkXHTTPServerParse isolates server-side metadata and padding
// validation for the default query-in-Referer padding.
func BenchmarkXHTTPServerParse(b *testing.B) {
	config := benchmarkConfig(b, option.V2RayXHTTPOptions{Path: "/xhttp", Mode: "packet-up"})
	request := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/xhttp/session01/42"}, Header: http.Header{"Referer": []string{"http://example.com/xhttp/session01?x_padding=XXXX"}}}
	b.Run("metadata", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			config.extractMetadata(request)
		}
	})
	b.Run("padding", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			config.validPadding(request)
		}
	})
}

// BenchmarkXHTTPParallel measures aggregate packet-up throughput across
// concurrent logical connections, where pacing on one connection overlaps
// with work on the others.
func BenchmarkXHTTPParallel(b *testing.B) {
	addr, closeServer := benchmarkServer(b, "packet-up")
	defer closeServer()
	options := option.V2RayXHTTPOptions{Path: "/xhttp", Mode: "packet-up", SCMinPostsIntervalMS: option.V2RayXHTTPRange{From: 1, To: 1}}
	dialer, err := dialer.NewDefault(context.Background(), option.DialerOptions{})
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(2 * 64 * 1024)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		client, err := NewClient(context.Background(), dialer, addr, options, nil)
		if err != nil {
			b.Error(err)
			return
		}
		defer client.Close()
		conn, err := client.DialContext(context.Background())
		if err != nil {
			b.Error(err)
			return
		}
		defer conn.Close()
		payload := make([]byte, 64*1024)
		response := make([]byte, len(payload))
		for pb.Next() {
			if _, err = conn.Write(payload); err != nil {
				b.Error(err)
				return
			}
			if _, err = io.ReadFull(conn, response); err != nil {
				b.Error(err)
				return
			}
		}
	})
}
