//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/config"
)

func TestDeprecatedFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	comp := &Component{
		opts: component.Options{
			Logger: logger,
		},
		args: Arguments{
			Port:           "8080",
			ExecutableName: "test-app",
			Metrics: config.Metrics{
				Features: []string{"network"},
			},
		},
	}

	comp.logDeprecationWarnings()

	output := buf.String()
	require.Contains(t, output, "level=WARN")
	require.Contains(t, output, "open_port' field is deprecated")
	require.Contains(t, output, "executable_name' field is deprecated")
}
