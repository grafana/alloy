package syslogparser

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"

	"github.com/leodido/go-syslog/v4"
	"github.com/stretchr/testify/require"
)

const delim = '\n'

func TestReadLineRaw_OctetCounting(t *testing.T) {
	cases := []struct {
		label      string
		inputFile  string
		expectFile string
	}{
		{
			label:      "multiline",
			inputFile:  "testdata/octetcount-multiline.txt",
			expectFile: "testdata/octetcount-multiline.json",
		},
		{
			label:      "singleline",
			inputFile:  "testdata/octetcount-singleline.txt",
			expectFile: "testdata/octetcount-singleline.json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			inputs, err := os.Open(tc.inputFile)
			require.NoError(t, err)
			t.Cleanup(func() { inputs.Close() })

			fexpects, err := os.Open(tc.expectFile)
			require.NoError(t, err)
			t.Cleanup(func() { fexpects.Close() })

			expects := []*syslog.Base{}
			err = json.NewDecoder(fexpects).Decode(&expects)
			require.NoError(t, err)

			i := 0
			for got, err := range IterStreamRaw(inputs, delim) {
				require.NoErrorf(t, err, "item: %d", i)
				expect := expects[i]
				require.Equalf(t, expect, got, "mismatch at index %d", i)
				i++
			}

			if i != len(expects) {
				t.Errorf("expected %d items, got %d", len(expects), i)
			}
		})
	}
}

func TestIterStreamRaw_NonTransparentTCP(t *testing.T) {
	inputs, err := os.Open("testdata/cisco-nontransparent.txt")
	require.NoError(t, err)
	t.Cleanup(func() { inputs.Close() })

	fexpects, err := os.Open("testdata/cisco-nontransparent.json")
	require.NoError(t, err)
	t.Cleanup(func() { fexpects.Close() })

	expects := []*syslog.Base{}
	err = json.NewDecoder(fexpects).Decode(&expects)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("failed to accept client connection: %v", err)
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(inputs)
		for scanner.Scan() {
			_, err = fmt.Fprintf(conn, "%s\n", scanner.Bytes())
			if err != nil {
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

	client, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	i := 0
	for got, err := range IterStreamRaw(client, delim) {
		require.NoErrorf(t, err, "item: %d", i)
		expect := expects[i]
		require.Equalf(t, expect, got, "mismatch at index %d", i)
		i++
	}

	if i != len(expects) {
		t.Errorf("expected %d items, got %d", len(expects), i)
	}
}
