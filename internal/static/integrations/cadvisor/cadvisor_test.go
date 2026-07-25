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

// TestConfig_StartsWithResctrlIncluded is the configuration reported in
// https://github.com/grafana/alloy/issues/5838. Listing disabled_metrics overrides the built-in
// default-disabled set, so the resctrl metric kind is collected, and starting cAdvisor's manager
// used to crash the process on machines that don't support resctrl.
func TestConfig_StartsWithResctrlIncluded(t *testing.T) {
	rawCfg := "docker_only: true\ndisabled_metrics: [disk, diskIO]"

	var cfg Config
	err := yaml.Unmarshal([]byte(rawCfg), &cfg)
	require.NoError(t, err)

	included, err := cfg.GetIncludedMetrics()
	require.NoError(t, err)
	require.True(t, included.Has(container.ResctrlMetrics), "this test is only meaningful while resctrl metrics are collected")

	ig, err := cfg.NewIntegration(util.TestAlloyLogger(t).Slog())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, ig.Run(ctx))
}
