package app_agent_receiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig_DefaultConfig(t *testing.T) {
	var cfg Config
	cb := `
test-conf: test-val`
	err := yaml.Unmarshal([]byte(cb), &cfg)
	require.NoError(t, err)
	require.Equal(t, []string(nil), cfg.Server.CORSAllowedOrigins)
	require.Equal(t, "127.0.0.1", cfg.Server.Host)
	require.Equal(t, 12347, cfg.Server.Port)
	require.Equal(t, true, cfg.Server.RateLimiting.Enabled)
}

func TestConfig_EnableRateLimitNoRPS(t *testing.T) {
	var cfg Config
	cb := `
server:
  rate_limiting:
    enabled: true`
	err := yaml.Unmarshal([]byte(cb), &cfg)
	require.NoError(t, err)
	require.Equal(t, true, cfg.Server.RateLimiting.Enabled)
	require.Equal(t, 100.0, cfg.Server.RateLimiting.RPS)
	require.Equal(t, 50, cfg.Server.RateLimiting.Burstiness)
}

func TestConfig_EnableRateLimitRPS(t *testing.T) {
	var cfg Config
	cb := `
server:
  rate_limiting:
    enabled: true
    rps: 142`
	err := yaml.Unmarshal([]byte(cb), &cfg)
	require.NoError(t, err)
	require.Equal(t, true, cfg.Server.RateLimiting.Enabled)
	require.Equal(t, 142.0, cfg.Server.RateLimiting.RPS)
	require.Equal(t, 50, cfg.Server.RateLimiting.Burstiness)
}

func TestConfig_MultipleUnmarshals(t *testing.T) {
	var cfg1 Config
	cb1 := `
sourcemaps:
  download_origins: ["one"]
logs_labels:
  app: frontend
  one: two`
	var cfg2 Config
	cb2 := `
logs_labels:
  app: backend
  bar: baz`

	err := yaml.UnmarshalStrict([]byte(cb1), &cfg1)
	require.NoError(t, err)
	err = yaml.UnmarshalStrict([]byte(cb2), &cfg2)
	require.NoError(t, err)

	require.Equal(t, map[string]string{
		"app": "frontend",
		"one": "two",
	}, cfg1.LogsLabels)
	require.Equal(t, []string{"one"}, cfg1.SourceMaps.DownloadFromOrigins)

	require.Equal(t, map[string]string{
		"app": "backend",
		"bar": "baz",
	}, cfg2.LogsLabels)
	require.Equal(t, []string{"*"}, cfg2.SourceMaps.DownloadFromOrigins)
}
