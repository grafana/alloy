package config

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/grafana/alloy/internal/static/integrations/install" // Install integrations for tests
	"github.com/grafana/alloy/internal/util"
)

func TestIntegrations_v1(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal

integrations:
  agent:
    enabled: true`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)
	require.NotNil(t, c.Integrations.ConfigV1)
}

func TestIntegrations_v2(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal

integrations:
  agent:
    autoscrape:
      enable: false`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test", "-enable-features=integrations-next"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)
	require.NotNil(t, c.Integrations.ConfigV2)
}

func TestSetVersionDoesNotOverrideExistingV1Integrations(t *testing.T) {
	cfg := `
integrations:
  agent:
    enabled: true`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Integrations.ConfigV1.Integrations))

	c.Integrations.raw = util.RawYAML{}
	c.Integrations.setVersion(IntegrationsVersion1)
	require.Equal(t, 1, len(c.Integrations.ConfigV1.Integrations))
}
