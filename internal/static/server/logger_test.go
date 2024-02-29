package server

import (
	"bytes"
	"testing"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestLogger_DefaultParameters(t *testing.T) {
	makeLogger := func(cfg *Config) (log.Logger, error) {
		var l log.Logger
		require.Equal(t, "info", cfg.LogLevel.String())
		require.Equal(t, "logfmt", cfg.LogFormat)
		return l, nil
	}
	defaultCfg := DefaultConfig()
	newLogger(&defaultCfg, makeLogger).makeLogger(&defaultCfg)
}

func TestLogger_ApplyConfig(t *testing.T) {
	var buf bytes.Buffer
	makeLogger := func(cfg *Config) (log.Logger, error) {
		l := log.NewLogfmtLogger(log.NewSyncWriter(&buf))
		if cfg.LogFormat == "json" {
			l = log.NewJSONLogger(log.NewSyncWriter(&buf))
		}
		l = level.NewFilter(l, cfg.LogLevel.Level.Option)
		return l, nil
	}

	var cfg Config
	cfgText := `log_level: error`

	err := yaml.Unmarshal([]byte(cfgText), &cfg)
	require.NoError(t, err)

	l := newLogger(&cfg, makeLogger)
	level.Debug(l).Log("msg", "this should not appear")

	cfgText = `
log_level: debug
log_format: json`
	err = yaml.Unmarshal([]byte(cfgText), &cfg)
	require.NoError(t, err)

	err = l.ApplyConfig(&cfg)
	require.NoError(t, err)

	level.Debug(l).Log("msg", "this should appear")
	require.JSONEq(t, `{
		"level":"debug",
		"msg":"this should appear"
	}`, buf.String())
}
