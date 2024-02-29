package automaticloggingprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v3"
)

func TestSpanKeyVals(t *testing.T) {
	tests := []struct {
		spanName  string
		spanAttrs map[string]interface{}
		spanStart time.Time
		spanEnd   time.Time
		cfg       AutomaticLoggingConfig
		expected  []interface{}
	}{
		{
			expected: []interface{}{
				"span", "",
				"dur", "0ns",
				"status", ptrace.StatusCode(1),
			},
		},
		{
			spanName: "test",
			expected: []interface{}{
				"span", "test",
				"dur", "0ns",
				"status", ptrace.StatusCode(1),
			},
		},
		{
			expected: []interface{}{
				"span", "",
				"dur", "0ns",
				"status", ptrace.StatusCode(1),
			},
		},
		{
			spanStart: time.Unix(0, 0),
			spanEnd:   time.Unix(0, 10),
			expected: []interface{}{
				"span", "",
				"dur", "10ns",
				"status", ptrace.StatusCode(1),
			},
		},
		{
			spanStart: time.Unix(0, 10),
			spanEnd:   time.Unix(0, 100),
			expected: []interface{}{
				"span", "",
				"dur", "90ns",
				"status", ptrace.StatusCode(1),
			},
		},
		{
			spanAttrs: map[string]interface{}{
				"xstr": "test",
			},
			expected: []interface{}{
				"span", "",
				"dur", "0ns",
				"status", ptrace.StatusCode(1),
			},
		},
		{
			spanAttrs: map[string]interface{}{
				"xstr": "test",
			},
			cfg: AutomaticLoggingConfig{
				SpanAttributes: []string{"xstr"},
			},
			expected: []interface{}{
				"span", "",
				"dur", "0ns",
				"status", ptrace.StatusCode(1),
				"xstr", "test",
			},
		},
		{
			cfg: AutomaticLoggingConfig{
				Overrides: OverrideConfig{
					SpanNameKey: "a",
					DurationKey: "c",
					StatusKey:   "d",
				},
			},
			expected: []interface{}{
				"a", "",
				"c", "0ns",
				"d", ptrace.StatusCode(1),
			},
		},
	}

	for _, tc := range tests {
		tc.cfg.Backend = BackendStdout
		tc.cfg.Spans = true
		p, err := newTraceProcessor(&automaticLoggingProcessor{}, &tc.cfg)
		require.NoError(t, err)

		span := ptrace.NewSpan()
		span.SetName(tc.spanName)
		span.Attributes().FromRaw(tc.spanAttrs)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(tc.spanStart))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(tc.spanEnd))
		span.Status().SetCode(ptrace.StatusCodeOk)

		actual := p.(*automaticLoggingProcessor).spanKeyVals(span)
		assert.Equal(t, tc.expected, actual)
	}
}

func TestProcessKeyVals(t *testing.T) {
	tests := []struct {
		processAttrs map[string]interface{}
		svc          string
		cfg          AutomaticLoggingConfig
		expected     []interface{}
	}{
		{
			expected: []interface{}{
				"svc", "",
			},
		},
		{
			processAttrs: map[string]interface{}{
				"xstr": "test",
			},
			expected: []interface{}{
				"svc", "",
			},
		},
		{
			processAttrs: map[string]interface{}{
				"xstr": "test",
			},
			cfg: AutomaticLoggingConfig{
				ProcessAttributes: []string{"xstr"},
			},
			expected: []interface{}{
				"svc", "",
				"xstr", "test",
			},
		},
	}

	for _, tc := range tests {
		tc.cfg.Backend = BackendStdout
		tc.cfg.Spans = true
		p, err := newTraceProcessor(&automaticLoggingProcessor{}, &tc.cfg)
		require.NoError(t, err)

		process := pcommon.NewResource()
		process.Attributes().FromRaw(tc.processAttrs)

		actual := p.(*automaticLoggingProcessor).processKeyVals(process, tc.svc)
		assert.Equal(t, tc.expected, actual)
	}
}

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

	err = p.Start(context.Background(), componenttest.NewNopHost())
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

func TestLabels(t *testing.T) {
	tests := []struct {
		name           string
		labels         []string
		keyValues      []interface{}
		expectedLabels model.LabelSet
	}{
		{
			name:      "happy case",
			labels:    []string{"loki", "svc"},
			keyValues: []interface{}{"loki", "loki", "svc", "gateway", "duration", "1s"},
			expectedLabels: map[model.LabelName]model.LabelValue{
				"loki": "loki",
				"svc":  "gateway",
			},
		},
		{
			name:      "happy case with dots",
			labels:    []string{"loki", "service.name"},
			keyValues: []interface{}{"loki", "loki", "service.name", "gateway", "duration", "1s"},
			expectedLabels: map[model.LabelName]model.LabelValue{
				"loki":         "loki",
				"service_name": "gateway",
			},
		},
		{
			name:           "no labels",
			labels:         []string{},
			keyValues:      []interface{}{"loki", "loki", "svc", "gateway", "duration", "1s"},
			expectedLabels: map[model.LabelName]model.LabelValue{},
		},
		{
			name:      "label not present in keyValues",
			labels:    []string{"loki", "svc"},
			keyValues: []interface{}{"loki", "loki", "duration", "1s"},
			expectedLabels: map[model.LabelName]model.LabelValue{
				"loki": "loki",
			},
		},
		{
			name:      "label value is not type string",
			labels:    []string{"loki"},
			keyValues: []interface{}{"loki", 42, "duration", "1s"},
			expectedLabels: map[model.LabelName]model.LabelValue{
				"loki": "42",
			},
		},
		{
			name:      "stringifies value if possible",
			labels:    []string{"status"},
			keyValues: []interface{}{"status", ptrace.StatusCode(1)},
			expectedLabels: map[model.LabelName]model.LabelValue{
				"status": model.LabelValue(ptrace.StatusCode(1).String()),
			},
		},
		{
			name:           "no keyValues",
			labels:         []string{"status"},
			keyValues:      []interface{}{},
			expectedLabels: map[model.LabelName]model.LabelValue{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &AutomaticLoggingConfig{
				Spans:  true,
				Labels: tc.labels,
			}
			p, err := newTraceProcessor(&automaticLoggingProcessor{}, cfg)
			require.NoError(t, err)

			ls := p.(*automaticLoggingProcessor).spanLabels(tc.keyValues)
			assert.Equal(t, tc.expectedLabels, ls)
		})
	}
}
