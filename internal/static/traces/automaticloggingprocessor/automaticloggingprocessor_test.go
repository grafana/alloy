package automaticloggingprocessor

import (
	"testing"

	"github.com/grafana/alloy/internal/static/logs"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"gopkg.in/yaml.v3"
)

func TestBadConfigs(t *testing.T) {
	tests := []struct {
		cfg *AutomaticLoggingConfig
	}{
		{
			cfg: &AutomaticLoggingConfig{},
		},
		{
			cfg: &AutomaticLoggingConfig{
				Backend: "blarg",
				Spans:   true,
			},
		},
		{
			cfg: &AutomaticLoggingConfig{
				Backend: "logs",
			},
		},
		{
			cfg: &AutomaticLoggingConfig{
				Backend: "loki",
			},
		},
		{
			cfg: &AutomaticLoggingConfig{
				Backend: "stdout",
			},
		},
	}

	for _, tc := range tests {
		p, err := newTraceProcessor(&automaticLoggingProcessor{}, tc.cfg)
		require.Error(t, err)
		require.Nil(t, p)
	}
}

func TestLogToStdoutSet(t *testing.T) {
	cfg := &AutomaticLoggingConfig{
		Backend: BackendStdout,
		Spans:   true,
	}

	p, err := newTraceProcessor(&automaticLoggingProcessor{}, cfg)
	require.NoError(t, err)
	require.True(t, p.(*automaticLoggingProcessor).logToStdout)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	cfg = &AutomaticLoggingConfig{
		Backend: BackendLogs,
		Spans:   true,
	}

	p, err = newTraceProcessor(&automaticLoggingProcessor{}, cfg)
	require.NoError(t, err)
	require.False(t, p.(*automaticLoggingProcessor).logToStdout)
}

func TestDefaults(t *testing.T) {
	cfg := &AutomaticLoggingConfig{
		Spans: true,
	}

	p, err := newTraceProcessor(&automaticLoggingProcessor{}, cfg)
	require.NoError(t, err)
	require.Equal(t, BackendStdout, p.(*automaticLoggingProcessor).cfg.Backend)
	require.Equal(t, defaultTimeout, p.(*automaticLoggingProcessor).cfg.Timeout)
	require.True(t, p.(*automaticLoggingProcessor).logToStdout)

	require.Equal(t, defaultLogsTag, p.(*automaticLoggingProcessor).cfg.Overrides.LogsTag)
	require.Equal(t, defaultServiceKey, p.(*automaticLoggingProcessor).cfg.Overrides.ServiceKey)
	require.Equal(t, defaultSpanNameKey, p.(*automaticLoggingProcessor).cfg.Overrides.SpanNameKey)
	require.Equal(t, defaultStatusKey, p.(*automaticLoggingProcessor).cfg.Overrides.StatusKey)
	require.Equal(t, defaultDurationKey, p.(*automaticLoggingProcessor).cfg.Overrides.DurationKey)
	require.Equal(t, defaultTraceIDKey, p.(*automaticLoggingProcessor).cfg.Overrides.TraceIDKey)
}

func TestLokiNameMigration(t *testing.T) {
	logsConfig := &logs.Config{
		Configs: []*logs.InstanceConfig{{Name: "default"}},
	}

	input := util.Untab(`
		backend: loki
		loki_name: default
		overrides:
			loki_tag: traces
	`)
	expect := util.Untab(`
		backend: logs_instance
		logs_instance_name: default
		overrides:
			logs_instance_tag: traces
	`)

	var cfg AutomaticLoggingConfig
	require.NoError(t, yaml.Unmarshal([]byte(input), &cfg))
	require.NoError(t, cfg.Validate(logsConfig))

	bb, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.YAMLEq(t, expect, string(bb))
}
