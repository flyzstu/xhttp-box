//go:build with_quic

package main

import (
	"context"
	std_tls "crypto/tls"
	"net"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

const (
	perfPayloadTotal = 1 << 28 // 256 MiB per direction
	perfPayloadChunk = 64 * 1024
	perfUDPCount     = 20000
	perfUDPSize      = 1024
	perfUDPRate      = 2000 // datagrams per second, sustainable for relay chains
	perfMinMBps      = 32   // relaxed floor for shared/CI runners
	perfUsername     = "perf-user"
	perfUserPassword = "perf-password"
)

// perfTCPThroughput measures full-duplex throughput over an echo connection.
func perfTCPThroughput(t *testing.T, name string, conn net.Conn) {
	t.Helper()
	defer conn.Close()
	payload := make([]byte, perfPayloadChunk)
	for i := range payload {
		payload[i] = byte(i)
	}
	received := make(chan int64, 1)
	go func() {
		var total int64
		buffer := make([]byte, 128*1024)
		for {
			n, err := conn.Read(buffer)
			total += int64(n)
			if err != nil || total >= perfPayloadTotal {
				received <- total
				return
			}
		}
	}()
	start := time.Now()
	var sent int64
	for sent < perfPayloadTotal {
		n, err := conn.Write(payload)
		require.NoError(t, err)
		sent += int64(n)
	}
	total := <-received
	elapsed := time.Since(start)
	throughput := float64(total+sent) / elapsed.Seconds() / 1024 / 1024
	t.Logf("%s: sent %.1f MiB, echoed %.1f MiB in %s, throughput %.1f MB/s",
		name, float64(sent)/1024/1024, float64(total)/1024/1024, elapsed, throughput)
	require.GreaterOrEqual(t, throughput, float64(perfMinMBps))
}

// startPerfUDPEchoServer starts a UDP echo loop on a fixed reserved port.
func startPerfUDPEchoServer(t *testing.T, port uint16) func() {
	t.Helper()
	echo, err := net.ListenPacket("udp", ":"+strconv.Itoa(int(port)))
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, perfUDPSize)
		for {
			n, source, readErr := echo.ReadFrom(buffer)
			if readErr != nil {
				return
			}
			t.Logf("echo received %d bytes from %s", n, source)
			if _, writeErr := echo.WriteTo(buffer[:n], source); writeErr != nil {
				return
			}
		}
	}()
	return func() {
		echo.Close()
		<-done
	}
}

// perfUDPThroughput measures datagram round-trip delivery with a concurrent
// sender and receiver (a small reader regardless of transport pacing).
func perfUDPThroughput(t *testing.T, name string, conn net.PacketConn, destination M.Socksaddr) {
	t.Helper()
	payload := make([]byte, perfUDPSize)
	for i := range payload {
		payload[i] = byte(i)
	}
	var received atomic.Int64
	var malformed atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, perfUDPSize+128)
		for {
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(30*time.Second)))
			n, _, err := conn.ReadFrom(buffer)
			if err != nil {
				return
			}
			if n < perfUDPSize-64 || n > perfUDPSize+64 {
				malformed.Add(1)
				continue
			}
			if received.Add(1) >= perfUDPCount {
				return
			}
		}
	}()
	start := time.Now()
	tick := time.NewTicker(time.Second / perfUDPRate)
	defer tick.Stop()
	sent := 0
	for sent < perfUDPCount {
		_, err := conn.WriteTo(payload, destination.UDPAddr())
		require.NoError(t, err)
		sent++
		<-tick.C
	}
	<-done
	conn.SetReadDeadline(time.Now())
	<-done
	elapsed := time.Since(start)
	delivered := received.Load()
	ratio := float64(delivered) / perfUDPCount
	mbps := float64(delivered) * perfUDPSize * 2 / elapsed.Seconds() / 1024 / 1024
	t.Logf("%s: delivered %d/%d datagrams (%d malformed) in %s (%.1f%%), throughput %.1f MB/s",
		name, delivered, perfUDPCount, malformed.Load(), elapsed, ratio*100, mbps)
	require.GreaterOrEqual(t, ratio, 0.95)
}

type perfStaticDialer struct {
	tlsConn net.Conn
}

func (d *perfStaticDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return d.tlsConn, nil
	default:
		return nil, E.New("TCP only")
	}
}

func (d *perfStaticDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("TCP only")
}

// perfSocksTLSClient returns a socks5 client speaking over a TLS connection
// to the given mixed TLS inbound.
func perfSocksTLSClient(t *testing.T, port uint16, username, password string) *socks.Client {
	t.Helper()
	tlsConn, err := std_tls.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(port)), &std_tls.Config{
		ServerName:         "example.org",
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		tlsConn.Close()
	})
	return socks.NewClient(&perfStaticDialer{tlsConn: tlsConn}, M.ParseSocksaddrHostPort("127.0.0.1", port), socks.Version5, username, password)
}

// startPerfHTTPTunnelChain starts two sing-box instances:
//
// socks-tls client -> [A: mixed TLS in + http(h2 TLS out)] -> [B: mixed TLS in + direct] -> echo
func startPerfHTTPTunnelChain(t *testing.T) (mixedInPort uint16, echoPort uint16) {
	t.Helper()
	_, certA, keyA := createSelfSignedCertificate(t, "example.org")
	_, certB, keyB := createSelfSignedCertificate(t, "example.org")
	mixedInPort = reserveOpenVPNTCPPort(t)
	httpOutPort := reserveOpenVPNTCPPort(t)
	echoPort = reserveOpenVPNEchoPort(t)

	users := []auth.User{{Username: perfUsername, Password: perfUserPassword}}
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-tls-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
						ListenPort: mixedInPort,
					},
					Users: users,
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certA,
							KeyPath:         keyA,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeHTTP,
				Tag:  "http-h2-out",
				Options: &option.HTTPOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: httpOutPort,
					},
					Username: perfUsername,
					Password: perfUserPassword,
					Version:  2,
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certB,
							Insecure:        true,
						},
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-tls-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "http-h2-out",
							},
						},
					},
				},
			},
		},
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-tls-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
						ListenPort: httpOutPort,
					},
					Users: users,
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certB,
							KeyPath:         keyB,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	return mixedInPort, echoPort
}

// startPerfMixedUDPChain starts one instance: socks UDP in -> mixed? (plain SOCKS UDP relay to direct)
func startPerfDirectUDPChain(t *testing.T) (proxyPort uint16) {
	t.Helper()
	proxyPort = reserveOpenVPNTCPPort(t)
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeSOCKS,
				Tag:  "socks-in",
				Options: &option.SocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
						ListenPort: proxyPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	return proxyPort
}

func TestPerfDirectUDP(t *testing.T) {
	proxyPort := startPerfDirectUDPChain(t)
	echoPort := reserveOpenVPNUDPPort(t)
	closeEcho := startPerfUDPEchoServer(t, echoPort)
	defer closeEcho()

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
	udpConn, err := dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	perfUDPThroughput(t, "direct udp (baseline)", udpConn, M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
}

func TestPerfMASQUETunnelTCP(t *testing.T) {
	environment := startMASQUE(t, nil, 3, true, 1500, nil, nil)
	readinessPort := reserveOpenVPNEchoPort(t)
	closeEcho := startOpenVPNReadinessEcho(t, readinessPort)
	defer closeEcho()
	waitForOpenVPNClientReady(t, environment.clientProxyPort, readinessPort, masqueServerAddress)

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", environment.clientProxyPort), socks.Version5, "", "")
	conn, err := dialer.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort(masqueServerAddress, readinessPort))
	require.NoError(t, err)
	perfTCPThroughput(t, "masque h3 tunnel TCP", conn)
}

func TestPerfMASQUETunnelUDP(t *testing.T) {
	environment := startMASQUE(t, nil, 3, true, 1500, nil, nil)
	echoPort := reserveOpenVPNUDPPort(t)
	closeEcho := startPerfUDPEchoServer(t, echoPort)
	defer closeEcho()
	waitForOpenVPNClientReady(t, environment.clientProxyPort, reserveOpenVPNEchoPort(t), masqueServerAddress)

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", environment.clientProxyPort), socks.Version5, "", "")
	udpConn, err := dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort(masqueServerAddress, echoPort))
	require.NoError(t, err)
	perfUDPThroughput(t, "masque h3 tunnel UDP", udpConn, M.ParseSocksaddrHostPort(masqueServerAddress, echoPort))
}

func TestPerfHTTPTunnelTCP(t *testing.T) {
	mixedInPort, echoPort := startPerfHTTPTunnelChain(t)
	closeEcho := startOpenVPNReadinessEcho(t, echoPort)
	defer closeEcho()
	client := perfSocksTLSClient(t, mixedInPort, perfUsername, perfUserPassword)
	conn, err := client.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", echoPort))
	require.NoError(t, err)
	perfTCPThroughput(t, "http h2 tunnel chain TCP", conn)
}
