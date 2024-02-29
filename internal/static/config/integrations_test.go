package config

import (
	"flag"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/grafana/agent/internal/static/integrations/install" // Install integrations for tests
	"github.com/grafana/agent/internal/util"
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

func TestEnabledIntegrations_v1(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal

integrations:
  agent:
    enabled: true
  node_exporter:
    enabled: true`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)

	actual := c.Integrations.EnabledIntegrations()
	sort.Strings(actual)
	expected := []string{"agent", "node_exporter"}
	sort.Strings(expected)
	require.Equal(t, actual, expected)
}

func TestEnabledIntegrations_v2(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal

integrations:
  agent:
    autoscrape:
      enable: false
  node_exporter:
    autoscrape:
      enable: false`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test", "-enable-features=integrations-next"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)

	actual := c.Integrations.EnabledIntegrations()
	sort.Strings(actual)
	expected := []string{"agent", "node_exporter"}
	sort.Strings(expected)
	require.Equal(t, actual, expected)
}

func TestEnabledIntegrations_v2MultipleInstances(t *testing.T) {
	cfg := `
metrics:
  wal_directory: /tmp/wal

integrations: 
  redis_configs:
  - redis_addr: "redis-0:6379"
  - redis_addr: "redis-1:6379"`

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	c, err := LoadFromFunc(fs, []string{"-config.file", "test", "-enable-features=integrations-next"}, func(_, _ string, _ bool, c *Config) error {
		return LoadBytes([]byte(cfg), false, c)
	})
	require.NoError(t, err)
	require.Equal(t, c.Integrations.EnabledIntegrations(), []string{"redis"})
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
