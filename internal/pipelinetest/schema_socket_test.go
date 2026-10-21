package pipelinetest

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateSocketInputs(t *testing.T) {
	tests := []struct {
		name    string
		tcp     []TCPInputSchema
		udp     []UDPInputSchema
		wantErr string
	}{
		{
			name: "valid",
			tcp:  []TCPInputSchema{{Address: "127.0.0.1:1514", Data: "x"}},
			udp:  []UDPInputSchema{{Address: "127.0.0.1:1514", Datagrams: []string{"x"}}},
		},
		{name: "tcp without port", tcp: []TCPInputSchema{{Address: "127.0.0.1", Data: "x"}}, wantErr: "tcp input 0: address must be host:port"},
		{name: "tcp without data", tcp: []TCPInputSchema{{Address: "127.0.0.1:1514"}}, wantErr: "tcp input 0: data is required"},
		{name: "udp without port", udp: []UDPInputSchema{{Address: "localhost", Datagrams: []string{"x"}}}, wantErr: "udp input 0: address must be host:port"},
		{name: "udp without datagrams", udp: []UDPInputSchema{{Address: "127.0.0.1:1514"}}, wantErr: "udp input 0: at least one datagram is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSocketInputs(tc.tcp, tc.udp)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// freeTCPAddr returns a local address that nothing listens on.
func freeTCPAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// TestProduceTCPInputs checks that the data is written as is over one
// connection, after waiting for a listener that starts late.
func TestProduceTCPInputs(t *testing.T) {
	addr := freeTCPAddr(t)
	got := make(chan string, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			got <- "listen: " + err.Error()
			return
		}
		defer l.Close()
		conn, err := l.Accept()
		if err != nil {
			got <- "accept: " + err.Error()
			return
		}
		defer conn.Close()
		data, _ := io.ReadAll(conn)
		got <- string(data)
	}()

	data := "line one\nline two\n"
	require.NoError(t, produceTCPInputs(t.Context(), []TCPInputSchema{{Address: addr, Data: data}}))
	require.Equal(t, data, <-got)
}

// TestProduceUDPInputs checks that each datagram arrives as its own packet.
func TestProduceUDPInputs(t *testing.T) {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	require.NoError(t, produceUDPInputs(t.Context(), []UDPInputSchema{{Address: c.LocalAddr().String(), Datagrams: []string{"first", "second"}}}))

	var packets []string
	buf := make([]byte, 1024)
	for range 2 {
		require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
		n, _, err := c.ReadFrom(buf)
		require.NoError(t, err)
		packets = append(packets, string(buf[:n]))
	}
	require.Equal(t, []string{"first", "second"}, packets)
}
