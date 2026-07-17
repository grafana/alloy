//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
	"github.com/grafana/alloy/syntax"
)

// TestSectionsEmitted asserts every config section (injector, javaagent,
// nodejs, …) reaches the emitted Beyla YAML, and that a *bool default-true
// field can be explicitly disabled.
func TestSectionsEmitted(t *testing.T) {
	cfg := `
		injector {
			instrument {
				open_ports = "8080"
				exe_path   = "*java"
			}
			enabled_sdks           = ["java"]
			exporter_otlp_endpoint = "http://alloy:4318"
			image_version          = "1.2.3"
			otel_exported_signals {
				metrics = false
			}
		}
		javaagent {
			enabled = true
		}
		nodejs {
			enabled = true
		}
		output { /* no-op */ }
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	comp := &Component{
		opts:       component.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		args:       args,
		subprocess: subprocess.New(),
	}
	comp.subprocess.SetListen(12345, "")

	configPath, cleanup, err := comp.writeConfigFile()
	require.NoError(t, err)
	defer cleanup()
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(data, &config))

	// All config sections must reach the emitted YAML.
	require.Contains(t, config, "injector")
	require.Contains(t, config, "javaagent")
	require.Contains(t, config, "nodejs")

	// Beyla 3.22 injector fields: enabled_sdks ([]InstrumentableType via scalar_types)
	// and exporter_otlp_endpoint (renamed from otel_endpoint) reach the YAML.
	inj := config["injector"].(map[string]any)
	require.Equal(t, []any{"java"}, inj["enabled_sdks"])
	require.Equal(t, "http://alloy:4318", inj["exporter_otlp_endpoint"])

	// *bool default-true field can be explicitly disabled.
	sig := inj["otel_exported_signals"].(map[string]any)
	require.Equal(t, false, sig["metrics"])
}
