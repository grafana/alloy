package stages

import (
	"testing"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestLogfmtStage(t *testing.T) {
	var (
		now               = time.Now()
		testLogfmtLogLine = `
			time=2012-11-01T22:08:41+00:00 app=loki	level=WARN duration=125 message="this is a log line" extra="user=foo""
		`
		testLogfmtLogFixture = `
			time=2012-11-01T22:08:41+00:00
			app=loki
			level=WARN
			nested="child=value"
			message="this is a log line"
		`
	)

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "decode logfmt on entry",
			config: `
			stage.logfmt {
				mapping = { "time" = "", "app" = "", "level" = "", "nested" = "", "message" = "" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"time":    "2012-11-01T22:08:41+00:00",
					"app":     "loki",
					"level":   "WARN",
					"nested":  "child=value",
					"message": "this is a log line",
				}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
		},
		{
			name: "decode logfmt on extracted source",
			config: `
			stage.logfmt {
				mapping = { "time" = "", "app" = "", "level" = "", "nested" = "", "message" = "" }
				source  = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"log": testLogfmtLogFixture}, model.LabelSet{}, "{}", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"time":    "2012-11-01T22:08:41+00:00",
					"app":     "loki",
					"level":   "WARN",
					"nested":  "child=value",
					"message": "this is a log line",
					"log":     testLogfmtLogFixture,
				}, model.LabelSet{}, "{}", now),
			},
		},
		{
			name: "missing extracted source",
			config: `
			stage.logfmt {
				mapping = { "app" = "" }
				source  = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
		},
		{
			name: "invalid logfmt on entry",
			config: `
			stage.logfmt {
				mapping = { "expr1" = "" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"invalid":"logfmt"}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"invalid":"logfmt"}`, now),
			},
		},
		{
			name: "invalid logfmt on extracted source",
			config: `
			stage.logfmt {
				mapping = { "app" = "" }
				source  = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"log": "not logfmt"}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"log": "not logfmt"}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
		},
		{
			name: "nil source",
			config: `
			stage.logfmt {
				mapping = { "app" = "" }
				source  = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"log": nil}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"log": nil}, model.LabelSet{}, testLogfmtLogFixture, now),
			},
		},
		{
			name: "single stage without source",
			config: `
			stage.logfmt {
				mapping = { "out" = "message", "app" = "", "duration" = "", "unknown" = "" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testLogfmtLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"out":      "this is a log line",
					"app":      "loki",
					"duration": "125",
				}, model.LabelSet{}, testLogfmtLogLine, now),
			},
		},
		{
			name: "multiple stages with source",
			config: `
			stage.logfmt {
				mapping = { "extra" = "" }
			}

			stage.logfmt {
				mapping = { "user" = "" }
				source  = "extra"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testLogfmtLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"extra": "user=foo",
					"user":  "foo",
				}, model.LabelSet{}, testLogfmtLogLine, now),
			},
		},
		{
			name: "regex",
			config: `
			stage.logfmt {
				regex = "pod_.*"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `time=2012-11-01T22:08:41+00:00 pod_name=my-pod-123 pod_label=my-label`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"pod_name":  "my-pod-123",
					"pod_label": "my-label",
				}, model.LabelSet{}, `time=2012-11-01T22:08:41+00:00 pod_name=my-pod-123 pod_label=my-label`, now),
			},
		},
		{
			name: "regex matching everything",
			config: `
			stage.logfmt {
				regex = ".*"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testLogfmtLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"time":     "2012-11-01T22:08:41+00:00",
					"app":      "loki",
					"level":    "WARN",
					"duration": "125",
					"message":  "this is a log line",
					"extra":    "user=foo",
				}, model.LabelSet{}, testLogfmtLogLine, now),
			},
		},
		{
			name: "expressions and regex",
			config: `
			stage.logfmt {
				mapping = { "out" = "message", "app" = "" }
				regex   = "(app|duration)"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testLogfmtLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"out":      "this is a log line",
					"app":      "loki",
					"duration": "125",
				}, model.LabelSet{}, testLogfmtLogLine, now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected)
		})
	}
}

func TestValidateLogfmtConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    string
		expectErr bool
	}{
		{
			name:   "valid mapping without source",
			config: `mapping = { "foo1" = "foo", "foo2" = "" }`,
		},
		{
			name: "valid mapping with source",
			config: `
			mapping = { "foo1" = "foo", "foo2" = "" }
			source  = "log"
			`,
		},
		{
			name:   "valid regex",
			config: `regex = "pod_.*"`,
		},
		{
			name: "valid mapping and regex",
			config: `
			mapping = { "foo1" = "foo" }
			regex   = "pod_.*"
			`,
		},
		{
			name:      "invalid regex returns err",
			config:    `regex = "[unclosed"`,
			expectErr: true,
		},
		{
			name:      "empty mapping returns err",
			config:    `mapping = {}`,
			expectErr: true,
		},
		{
			name:      "source only returns err",
			config:    `source = "log"`,
			expectErr: true,
		},
		{
			name:      "empty config returns err",
			config:    ``,
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg LogfmtConfig
			err := syntax.Unmarshal([]byte(tt.config), &cfg)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReverseMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mapping  map[string]string
		expected map[string]string
	}{
		{
			name:     "nil mapping",
			mapping:  nil,
			expected: map[string]string{},
		},
		{
			name:     "value is used as the logfmt key",
			mapping:  map[string]string{"out": "message"},
			expected: map[string]string{"message": "out"},
		},
		{
			name:     "empty value falls back to the extracted key",
			mapping:  map[string]string{"app": ""},
			expected: map[string]string{"app": "app"},
		},
		{
			name:     "mixed values",
			mapping:  map[string]string{"out": "message", "app": ""},
			expected: map[string]string{"message": "out", "app": "app"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expected, reverseMapping(tt.mapping))
		})
	}
}
