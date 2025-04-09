//go:build !nonetwork && !nodocker && linux

package cadvisor

import (
	"context"
	"testing"

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

		ig, err := cfg.NewIntegration(util.TestLogger(t))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, ig.Run(ctx))
	})
}
