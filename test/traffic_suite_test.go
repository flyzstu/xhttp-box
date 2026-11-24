package main

import (
	"context"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/stretchr/testify/require"
)

// Generic SOCKS-based traffic suite shared by tunnel endpoint tests
// (masque, perf). Extracted from the removed OpenVPN test helpers.

func testProxySuit(t *testing.T, proxyPort uint16, echoPort uint16, tunnelAddress string) {
	t.Helper()
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
	destination := M.ParseSocksaddrHostPort(tunnelAddress, echoPort)
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), N.NetworkTCP, destination)
	}
	dialUDP := func() (net.PacketConn, error) {
		conn, err := dialer.DialContext(context.Background(), N.NetworkUDP, destination)
		if err != nil {
			return nil, err
		}
		return bufio.NewUnbindPacketConn(conn), nil
	}
	require.NoError(t, testPingPongWithConn(t, echoPort, dialTCP))
	require.NoError(t, testPingPongWithPacketConn(t, echoPort, dialUDP))
	require.NoError(t, testLargeDataWithConn(t, echoPort, dialTCP))
}

func waitForProxyClientReady(t *testing.T, proxyPort uint16, echoPort uint16, tunnelAddress string) {
	t.Helper()
	closeEcho := startReadinessEcho(t, echoPort)
	defer closeEcho()
	waitForProxyRemoteReady(t, proxyPort, tunnelAddress, echoPort, 3*time.Minute)
}

func startReadinessEcho(t *testing.T, port uint16) func() {
	t.Helper()
	listener, err := listen("tcp", ":"+strconv.Itoa(int(port)))
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go echoTCPConnection(conn)
		}
	}()
	return func() {
		listener.Close()
		<-done
	}
}

func waitForProxyRemoteReady(t *testing.T, proxyPort uint16, tunnelAddress string, tunnelPort uint16, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = probeProxyTCPWithTimeout(proxyPort, tunnelAddress, tunnelPort, 3*time.Second)
		if lastErr == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, lastErr)
}

func probeProxyTCPWithTimeout(proxyPort uint16, tunnelAddress string, tunnelPort uint16, timeout time.Duration) error {
	resultCh := make(chan error, 1)
	go func() {
		probeErr := probeProxyTCP(proxyPort, tunnelAddress, tunnelPort)
		resultCh <- probeErr
	}()
	select {
	case resultErr := <-resultCh:
		return resultErr
	case <-time.After(timeout):
		return E.New("timeout")
	}
}

func probeProxyTCP(proxyPort uint16, tunnelAddress string, tunnelPort uint16) error {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
	destination := M.ParseSocksaddrHostPort(tunnelAddress, tunnelPort)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, N.NetworkTCP, destination)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(time.Second))
	if err != nil {
		return err
	}
	return writeAndReadEcho(conn, []byte("ready"))
}

func echoTCPConnection(conn net.Conn) {
	defer conn.Close()
	buffer := make([]byte, 64*1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		_, err = conn.Write(buffer[:n])
		if err != nil {
			return
		}
	}
}

func writeAndReadEcho(conn net.Conn, payload []byte) error {
	_, err := conn.Write(payload)
	if err != nil {
		return err
	}
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return err
	}
	if string(payload) != string(response) {
		return E.New("unexpected TCP echo response")
	}
	return nil
}
func reserveTCPPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	tcpAddress := listener.Addr().(*net.TCPAddr)
	return uint16(tcpAddress.Port)
}

func reserveUDPPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	udpAddress := listener.LocalAddr().(*net.UDPAddr)
	return uint16(udpAddress.Port)
}

func reserveEchoPort(t *testing.T) uint16 {
	t.Helper()
	for range 20 {
		tcpListener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		tcpAddress := tcpListener.Addr().(*net.TCPAddr)
		port := uint16(tcpAddress.Port)
		udpListener, err := net.ListenPacket("udp", ":"+strconv.Itoa(int(port)))
		if err == nil {
			udpListener.Close()
			tcpListener.Close()
			return port
		}
		tcpListener.Close()
	}
	t.Fatal("reserve TCP and UDP echo port")
	return 0
}
