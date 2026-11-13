//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/config"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
)

func TestDeprecatedFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	comp := &Component{
		opts: component.Options{
			Logger: logger,
		},
		//nolint:staticcheck // sets the deprecated fields to assert the warnings
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

func TestDeprecatedECSRefreshInterval(t *testing.T) {
	var buf bytes.Buffer
	comp := &Component{
		opts: component.Options{Logger: slog.New(slog.NewTextHandler(&buf, nil))},
		//nolint:staticcheck // sets the deprecated field to assert the warning
		args: Arguments{NameResolver: config.NameResolver{ECS: config.ECSNameResolver{RefreshInterval: time.Minute}}},
	}

	comp.logDeprecationWarnings()

	require.Contains(t, buf.String(), "name_resolver.ecs.refresh_interval is deprecated")
}

func TestBuildConfigRedactsDebugLog(t *testing.T) {
	var buf bytes.Buffer
	comp := &Component{
		opts:       component.Options{Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))},
		subprocess: subprocess.New(),
	}
	comp.subprocess.SetListen(12345, "")
	tls := config.PeerMetadataTLS{Enabled: true, CertFile: "tls.crt", KeyFile: "tls.key"}

	configData, err := comp.buildConfig(Arguments{PeerMetadata: config.PeerMetadata{Enabled: true, Token: "test-token", TLS: tls}})
	require.NoError(t, err)

	require.Contains(t, string(configData), "test-token", "Beyla needs the token")
	require.Contains(t, buf.String(), "generated Beyla YAML config")
	require.Contains(t, buf.String(), "(secret)")
	require.NotContains(t, buf.String(), "test-token")
}
