package syslogparser

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/leodido/go-syslog/v4"
	"github.com/stretchr/testify/require"
)

const delim = '\n'

func TestReadLineRaw_OctetCounting(t *testing.T) {

}

func TestReadLineRaw_NonTransparentUDP(t *testing.T) {
	inputs, err := os.Open("testdata/cisco-nontransparent.txt")
	require.NoError(t, err)
	t.Cleanup(func() { inputs.Close() })

	fexpects, err := os.Open("testdata/cisco-nontransparent.json")
	require.NoError(t, err)
	t.Cleanup(func() { fexpects.Close() })

	expects := []*syslog.Base{}
	err = json.NewDecoder(fexpects).Decode(&expects)
	require.NoError(t, err)

	server, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { server.Close() })

	serverAddr := server.LocalAddr().(*net.UDPAddr)
	client, err := net.DialUDP("udp", nil, serverAddr)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	go func() {
		defer server.Close()

		scanner := bufio.NewScanner(inputs)
		clientAddr := client.LocalAddr().(*net.UDPAddr)
		for scanner.Scan() {
			line := append([]byte{}, scanner.Bytes()...)
			line = append(line, delim)

			if _, err := server.WriteToUDP(line, clientAddr); err != nil {
				t.Errorf("failed to write to client: %v", err)
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			t.Errorf("failed to scan inputs: %v", err)
		}
	}()

	for {
		if len(expects) == 0 {
			break
		}

		expect := expects[0]
		expects = expects[1:]
		got := readLineWithTimeout(t, client, delim, 10*time.Second)
		require.NoError(t, err)
		require.Equal(t, expect, got)
	}
}

type result struct {
	b   *syslog.Base
	err error
}

func readLineWithTimeout(t *testing.T, r io.ReadCloser, delim byte, timeout time.Duration) *syslog.Base {
	results := make(chan result, 1)
	go func() {
		got, err := ReadLineRaw(r, delim)
		results <- result{b: got, err: err}
		close(results)
	}()

	ctx, cancelFn := context.WithTimeout(t.Context(), timeout)
	defer cancelFn()
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatal("timeout exceeded")
		}
	case got := <-results:
		require.NoError(t, got.err)
		return got.b
	}
	return nil
}

func TestReadLineRaw_OctetCount(t *testing.T) {
	// TODO
}
