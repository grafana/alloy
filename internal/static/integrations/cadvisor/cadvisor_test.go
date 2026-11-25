//go:build !nonetwork && !nodocker && linux

package cadvisor

import (
	"context"
	"testing"

	"github.com/google/cadvisor/container"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfig_DockerOnly(t *testing.T) {
	t.Run("docker_only with default configuration is successful", func(t *testing.T) {
		// Run it once with the default config, expecting success.
		defaultCfg := `docker_only: true`

		var cfg Config
		err := yaml.Unmarshal([]byte(defaultCfg), &cfg)
		require.NoError(t, err)

		ig, err := cfg.NewIntegration(util.TestAlloyLogger(t).Slog())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, ig.Run(ctx))
	})
}

func TestConfig_GetIncludedMetrics(t *testing.T) {
	t.Run("default config keeps the built-in metrics disabled", func(t *testing.T) {
		var cfg Config
		included, err := cfg.GetIncludedMetrics()
		require.NoError(t, err)

		for metric := range defaultDisabledMetrics {
			require.Falsef(t, included.Has(metric), "expected %q to remain disabled by default", metric)
		}
	})

	t.Run("user-specified disabled_metrics is additive to the built-in defaults", func(t *testing.T) {
		cfg := Config{DisabledMetrics: []string{"disk", "diskIO"}}
		included, err := cfg.GetIncludedMetrics()
		require.NoError(t, err)

		require.False(t, included.Has(container.DiskUsageMetrics))
		require.False(t, included.Has(container.DiskIOMetrics))

		// The built-in defaults, e.g. ResctrlMetrics, must stay disabled too. Previously,
		// specifying disabled_metrics would reset the whole default-disabled set instead of
		// adding to it, silently re-enabling metrics like ResctrlMetrics which can panic with a
		// nil pointer dereference. See https://github.com/grafana/alloy/issues/5838.
		for metric := range defaultDisabledMetrics {
			require.Falsef(t, included.Has(metric), "expected %q to remain disabled after a partial override", metric)
		}
	})

	t.Run("calling GetIncludedMetrics repeatedly does not leak state across calls", func(t *testing.T) {
		narrow := Config{DisabledMetrics: []string{"disk"}}
		_, err := narrow.GetIncludedMetrics()
		require.NoError(t, err)

		var defaultCfg Config
		included, err := defaultCfg.GetIncludedMetrics()
		require.NoError(t, err)
		require.True(t, included.Has(container.DiskUsageMetrics), "a previous call's disabled_metrics must not leak into later calls")
	})
}
