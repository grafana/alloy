//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
)

// unixPathMax is the size of sun_path in the Linux kernel's sockaddr_un.
const unixPathMax = 108

func TestAbstractSocketAddr(t *testing.T) {
	// Long remotecfg pipeline names previously pushed the name past sun_path.
	longID := "remotecfg/beyla_k8s_appo11y_k8s_monitoring_sdk_injection.default/beyla.ebpf.default"

	tests := []struct {
		name        string
		role        string
		componentID string
	}{
		{name: "short component ID", role: "otlp", componentID: "beyla.ebpf.default"},
		{name: "long remotecfg component ID", role: "otlp", componentID: longID},
		{name: "long remotecfg component ID health", role: "health", componentID: longID},
		{name: "pathologically long component ID", role: "health", componentID: strings.Repeat("x", 500)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr := abstractSocketAddr(tc.role, tc.componentID)

			require.True(t, strings.HasPrefix(addr, "@alloy-beyla-"+tc.role+"-"))
			require.LessOrEqual(t, len(addr), unixPathMax)

			lis, err := net.Listen("unix", addr)
			require.NoError(t, err)
			require.NoError(t, lis.Close())
		})
	}
}

func TestBeylaSubprocessCapsIncludesDACOverride(t *testing.T) {
	require.Contains(t, beylaSubprocessCaps, uintptr(unix.CAP_DAC_OVERRIDE))
}

func TestDecompressBeylaBinary(t *testing.T) {
	enc, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	// Stand-in for the Beyla binary.
	// It contains arbitrary bytes that start with the magic number of ELF, "\x7fELF".
	// ELF is Executable and Linkable Format, the file format of Linux executables such as Beyla.
	// The test only checks that compressed bytes round-trip unchanged.
	exe := bytes.Repeat([]byte("\x7fELF not really a binary "), 1000)

	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{name: "zstd-compressed binary", input: enc.EncodeAll(exe, nil), want: exe},
		{name: "uncompressed binary", input: exe, wantErr: true},
		{name: "truncated archive", input: enc.EncodeAll(exe, nil)[:20], wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decompressBeylaBinary(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBeylaSpecPassesTheFilesWhereBeylaLooks(t *testing.T) {
	shell, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Skipf("no shell to run: %v", err)
	}

	control, controlSocket, err := newConfigControl()
	require.NoError(t, err)
	defer control.Close()

	configFile, err := writeConfigFile([]byte("discovery: {}\n"))
	require.NoError(t, err)

	comp := &Component{
		opts:       component.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		subprocess: subprocess.New(),
	}
	spec := comp.beylaSpec(shell, controlSocket, configFile)

	// The shell stands in for Beyla: it writes to the descriptor that the environment names
	// and prints the file that -config names.
	require.Equal(t, "-config", spec.Args[0])
	spec.Args = []string{"-c", `eval "echo ping >&$` + configControlEnv + `" && cat "$1"`, "beyla", spec.Args[1]}
	spec.PreExec = nil
	var stdout, stderr bytes.Buffer
	spec.Stdout, spec.Stderr = &stdout, &stderr

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	proc, err := subprocess.Start(ctx, spec)
	require.NoError(t, err)
	require.NoError(t, proc.Wait(), stderr.String())

	require.Equal(t, "discovery: {}\n", stdout.String())

	require.NoError(t, control.conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	received, err := bufio.NewReader(control.conn).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ping\n", received)
}
