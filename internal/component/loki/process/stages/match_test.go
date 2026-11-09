package stages

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

var (
	testMatchLogLineApp1 = `{"time":"2012-11-01T22:08:41+00:00", "app":"loki", "component": ["parser", "type"], "level": "WARN", "message": "app1 log line"}`
	testMatchLogLineApp2 = `{"time":"2012-11-01T22:08:41+00:00", "app":"poki", "component": ["parser", "type"], "level": "WARN", "msg" : "app2 log line"}`
)

func TestMatchStage(t *testing.T) {
	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	now := time.Now()

	tests := []testCase{
		{
			name: "keep routes matching entries through the nested pipeline",
			config: `
			stage.json {
				expressions = { "app" = "" }
			}

			stage.labels {
				values = { "app" = "" }
			}

			stage.match {
				selector = "{app=\"loki\"}"
				action = "keep"

				stage.json {
					expressions = { "msg" = "message" }
				}
			}

			stage.match {
				selector = "{app=\"poki\"}"
				action = "keep"

				stage.json {
					expressions = { "msg" = "msg" }
				}
			}

			stage.output {
				source = "msg"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testMatchLogLineApp1, now),
				newEntry(map[string]any{}, model.LabelSet{}, testMatchLogLineApp2, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"app": "loki", "msg": "app1 log line"}, model.LabelSet{"app": "loki"}, "app1 log line", now),
				newEntry(map[string]any{"app": "poki", "msg": "app2 log line"}, model.LabelSet{"app": "poki"}, "app2 log line", now),
			},
		},
		{
			name: `keep with matching selector and line filter runs the nested pipeline`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} |= \"foo\""
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "different"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"foo": "different", "test_label": "value"}, model.LabelSet{"foo": "different"}, "foo", now),
			},
		},
		{
			name: `keep with matching selector and regex line filter runs the nested pipeline`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} |~ \"foo\""
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "fooz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "different"}, "fooz", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "fooz", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"foo": "different", "test_label": "value"}, model.LabelSet{"foo": "different"}, "fooz", now),
			},
		},
		{
			name: `drop with matching selector and line filter drops matching entries`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} |= \"foo\""
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "different"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"foo": "different"}, model.LabelSet{"foo": "different"}, "foo", now),
			},
		},
		{
			name: `drop with matching selector and regex line filter drops matching entries`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} |~ \"foo\""
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "fooz", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "different"}, "fooz", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"foo": "different"}, model.LabelSet{"foo": "different"}, "fooz", now),
			},
		},
		{
			name: `keep with or line filter runs the nested pipeline when any operand matches`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} |= \"foo\" or \"baz\""
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "other", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "different"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "baz", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "other", now),
				newEntry(map[string]any{"foo": "different", "test_label": "value"}, model.LabelSet{"foo": "different"}, "foo", now),
			},
		},
		{
			name: `drop with or line filter drops entries when any operand matches`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} |= \"foo\" or \"baz\""
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "other", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "different"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar"}, model.LabelSet{"foo": "bar"}, "other", now),
				newEntry(map[string]any{"foo": "different"}, model.LabelSet{"foo": "different"}, "foo", now),
			},
		},
		{
			name: `keep with negated line filter runs the nested pipeline only for lines without the string`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} != \"foo\""
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "different"}, "baz", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "baz", now),
				newEntry(map[string]any{"foo": "different", "test_label": "value"}, model.LabelSet{"foo": "different"}, "baz", now),
			},
		},
		{
			name: `drop with negated line filter drops only lines without the string`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} != \"foo\""
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "different"}, "baz", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"foo": "different"}, model.LabelSet{"foo": "different"}, "baz", now),
			},
		},
		{
			name: `keep with negated regex line filter runs the nested pipeline only for lines the regex does not match`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} !~ \"fo+\""
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "fooz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "different"}, "baz", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "fooz", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "baz", now),
				newEntry(map[string]any{"foo": "different", "test_label": "value"}, model.LabelSet{"foo": "different"}, "baz", now),
			},
		},
		{
			name: `drop with negated regex line filter drops only lines the regex does not match`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} !~ \"fo+\""
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "fooz", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "different"}, "baz", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar"}, model.LabelSet{"foo": "bar"}, "fooz", now),
				newEntry(map[string]any{"foo": "different"}, model.LabelSet{"foo": "different"}, "baz", now),
			},
		},
		{
			name: `keep with negated or line filter runs the nested pipeline only for lines without any operand`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} != \"foo\" or \"baz\""
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "other", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "different"}, "other", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "baz", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "other", now),
				newEntry(map[string]any{"foo": "different", "test_label": "value"}, model.LabelSet{"foo": "different"}, "other", now),
			},
		},
		{
			name: `keep with chained line filters runs the nested pipeline only when all filters match`,
			config: `
			stage.match {
				selector = "{foo=\"bar\"} |= \"a\" != \"b\""
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "a", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "ab", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "c", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "different"}, "a", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "a", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "ab", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "c", now),
				newEntry(map[string]any{"foo": "different", "test_label": "value"}, model.LabelSet{"foo": "different"}, "a", now),
			},
		},
		{
			name: `keep with empty label matcher runs the nested pipeline only for entries with the label absent`,
			config: `
			stage.match {
				selector = "{foo=\"\"}"
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"test_label": "value"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
			},
		},
		{
			name: `drop with empty label matcher drops entries only when the label is absent`,
			config: `
			stage.match {
				selector = "{foo=\"\"}"
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar"}, model.LabelSet{"foo": "bar"}, "foo", now),
			},
		},
		{
			name: `keep with negated label matcher runs the nested pipeline only for entries with a different label value`,
			config: `
			stage.match {
				selector = "{foo!=\"bar\"}"
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "baz"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"foo": "baz", "test_label": "value"}, model.LabelSet{"foo": "baz", "test_label": "value"}, "foo", now),
			},
		},
		{
			name: `drop with negated label matcher drops entries with a different label value`,
			config: `
			stage.match {
				selector = "{foo!=\"bar\"}"
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "baz"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar"}, model.LabelSet{"foo": "bar"}, "foo", now),
			},
		},
		{
			name: `keep with multiple label matchers runs the nested pipeline only when all matchers match`,
			config: `
			stage.match {
				selector = "{foo=\"bar\",bar!=\"test\"}"
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "test_label": "value"}, model.LabelSet{"foo": "bar", "test_label": "value"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "bar": "test", "test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
		},
		{
			name: `drop with multiple label matchers drops entries only when all matchers match`,
			config: `
			stage.match {
				selector = "{foo=\"bar\",bar!=\"test\"}"
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "bar": "test"}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
		},
		{
			name: `keep with regex label matcher runs the nested pipeline only when the label value matches the regex`,
			config: `
			stage.match {
				selector = "{foo=\"bar\",bar=~\"te.*\"}"
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "other"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "bar": "test", "test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "test", "test_label": "value"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "bar": "other", "test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "other"}, "foo", now),
			},
		},
		{
			name: `drop with regex label matcher drops entries only when the label value matches the regex`,
			config: `
			stage.match {
				selector = "{foo=\"bar\",bar=~\"te.*\"}"
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar", "bar": "other"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "bar": "other"}, model.LabelSet{"foo": "bar", "bar": "other"}, "foo", now),
			},
		},
		{
			name: `keep with negated regex label matcher runs the nested pipeline only when the label value does not match the regex`,
			config: `
			stage.match {
				selector = "{foo=\"bar\",bar!~\"te.*\"}"
				action   = "keep"
				stage.labels {
					values = { "test_label" = "" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "other"}, "foo", now),
				newEntry(map[string]any{"test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "bar": "other", "test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "other", "test_label": "value"}, "foo", now),
				newEntry(map[string]any{"foo": "bar", "bar": "test", "test_label": "value"}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
		},
		{
			name: `drop with negated regex label matcher drops entries only when the label value does not match the regex`,
			config: `
			stage.match {
				selector = "{foo=\"bar\",bar!~\"te.*\"}"
				action   = "drop"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar", "bar": "other"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"foo": "bar", "bar": "test"}, model.LabelSet{"foo": "bar", "bar": "test"}, "foo", now),
			},
		},
		{
			name: `keep seeds extracted with labels added before the match for the nested pipeline`,
			config: `
			stage.static_labels {
				values = { "env" = "prod" }
			}

			stage.match {
				selector = "{app=\"loki\"}"
				action   = "keep"
				stage.labels {
					values = { "env_copy" = "env" }
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "foo", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "other"}, "foo", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"app": "loki", "env": "prod"}, model.LabelSet{"app": "loki", "env": "prod", "env_copy": "prod"}, "foo", now),
				newEntry(map[string]any{"app": "other"}, model.LabelSet{"app": "other", "env": "prod"}, "foo", now),
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

func TestMatchStageNestedPipelineError(t *testing.T) {
	cfgs := loadConfig(`
	stage.match {
		selector = "{app=\"loki\"}"
		action   = "keep"

		stage.timestamp {
			source = "timestamp"
			format = ""
		}
	}
	`)

	logger := util.TestAlloyLogger(t)
	next := func(_ context.Context, _ []Entry) error { return nil }
	_, err := newStageWithOpts(cfgs[0], stageOpts{
		slogger:      logger.Slog(),
		registerer:   prometheus.NewRegistry(),
		minStability: featuregate.StabilityGenerallyAvailable,
		next:         next,
	})
	require.ErrorContains(t, err, "match stage failed to create pipeline")
	require.ErrorContains(t, errors.Unwrap(err), "invalid stage config")
}

// TestMatchStageOrder asserts that entries are forwarded in timestamp order
// even though the stage runs the nested pipeline on only a subset of the batch.
func TestMatchStageOrder(t *testing.T) {
	now := time.Now()

	run := func(t *testing.T, cfgs []StageConfig, entries []Entry) []string {
		var collected []Entry
		next := func(_ context.Context, entries []Entry) error {
			collected = append(collected, entries...)
			return nil
		}

		p, err := newPipeline(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(t, err)
		require.NoError(t, p.process(context.Background(), entries))
		p.stop()

		got := make([]string, 0, len(collected))
		for _, e := range collected {
			got = append(got, e.Line)
		}
		return got
	}

	t.Run("line filter selector restores timestamp order", func(t *testing.T) {
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"loki\"} |= \"match\""

			stage.replace {
				expression = "(match)"
				replace    = "matched"
			}
		}
		`)

		got := run(t, cfgs, []Entry{
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "match 1", now),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "other 2", now.Add(1*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "match 3", now.Add(2*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "other 4", now.Add(3*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "match 5", now.Add(4*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "other 6", now.Add(5*time.Millisecond)),
		})

		require.Equal(t, []string{
			"matched 1",
			"other 2",
			"matched 3",
			"other 4",
			"matched 5",
			"other 6",
		}, got)
	})

	t.Run("label matcher selector restores timestamp order", func(t *testing.T) {
		// The nested stage rewrites app, so matched and bypassed entries end up
		// in the same stream even though the selector has no line filter.
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"loki\"}"

			stage.static_labels {
				values = { "app" = "other" }
			}
		}
		`)

		got := run(t, cfgs, []Entry{
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "line 1", now),
			newEntry(map[string]any{}, model.LabelSet{"app": "other"}, "line 2", now.Add(1*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "line 3", now.Add(2*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "other"}, "line 4", now.Add(3*time.Millisecond)),
		})

		require.Equal(t, []string{
			"line 1",
			"line 2",
			"line 3",
			"line 4",
		}, got)
	})

	t.Run("sorts entries when inner pipeline changes timestamp", func(t *testing.T) {
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"loki\"} |= \"match\""

			stage.regex {
				expression = "ts=(?P<ts>[0-9]+)"
			}
			stage.timestamp {
				source = "ts"
				format = "Unix"
			}
		}
		`)

		got := run(t, cfgs, []Entry{
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "match ts=2000", now),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "other 1", now.Add(1*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "match ts=1000", now.Add(2*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "other 2", now.Add(3*time.Millisecond)),
		})

		require.Equal(t, []string{
			"match ts=1000",
			"match ts=2000",
			"other 1",
			"other 2",
		}, got)
	})

	t.Run("entries sharing a timestamp keeps input order", func(t *testing.T) {
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"loki\"} |= \"match\""

			stage.replace {
				expression = "(match)"
				replace    = "matched"
			}
		}
		`)

		got := run(t, cfgs, []Entry{
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "match 1", now),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "other 2", now),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "other 3", now.Add(1*time.Millisecond)),
			newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "match 4", now.Add(1*time.Millisecond)),
		})

		require.Equal(t, []string{
			"matched 1",
			"other 2",
			"other 3",
			"matched 4",
		}, got)
	})
}

func TestValidateMatcherConfig(t *testing.T) {
	type testCase struct {
		name      string
		config    string
		expectErr bool
	}

	tests := []testCase{
		{
			name:      "empty config",
			config:    ``,
			expectErr: true,
		},
		{
			name: "missing selector",
			config: `
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name: "empty selector",
			config: `
				selector = ""
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name:      "keep action and no stages",
			config:    `selector = "{app=\"foo\"}"`,
			expectErr: true,
		},
		{
			name: "keep action with stages",
			config: `
				selector = "{app=\"foo\"}"
				action   = "keep"
				stage.label_drop {
					values = ["unused"]
				}
			`,
		},
		{
			name: "drop action with stages",
			config: `
				selector = "{app=\"foo\"}"
				action   = "drop"
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name: "drop action and no stages",
			config: `
				selector = "{app=\"foo\"}"
				action   = "drop"
			`,
		},
		{
			name: "bad action",
			config: `
				selector = "{app=\"foo\"}"
				action   = "nope"
			`,
			expectErr: true,
		},
		{
			name: "empty action with stages, defaults to keep",
			config: `
				selector = "{app=\"foo\"}"
				stage.label_drop {
					values = ["unused"]
				}
			`,
		},
		{
			name: "selector with unclosed quote",
			config: `
				selector = "{app=\"foo}"
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name: "selector missing braces",
			config: `
				selector = "foo"
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name: "selector with no matchers",
			config: `
				selector = "{}"
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name: "selector with unclosed brace",
			config: `
				selector = "{"
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name: "selector with invalid line filter regex",
			config: `
				selector = "{foo=\"bar\"} !~ \"[]\""
				stage.label_drop {
					values = ["unused"]
				}
			`,
			expectErr: true,
		},
		{
			name: "selector with line filter",
			config: `
				selector = "{foo=\"bar\"} |= \"foo\""
				stage.label_drop {
					values = ["unused"]
				}
			`,
		},
		{
			name: "selector with or line filter",
			config: `
				selector = "{foo=\"bar\"} |= \"foo\" or \"baz\""
				stage.label_drop {
					values = ["unused"]
				}
			`,
		},
		{
			name: "selector with parser and label filter",
			config: `
				selector = "{foo=\"bar\"} | json | status >= 200"
				stage.label_drop {
					values = ["unused"]
				}
			`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg MatchConfig
			err := syntax.Unmarshal([]byte(tt.config), &cfg)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func BenchmarkMatchStage(b *testing.B) {
	const numEntries = 50

	b.Run("all entries match", func(b *testing.B) {
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"bench\"}"

			stage.label_drop {
				values = ["unused"]
			}
		}
		`)

		entries := make([]push.Entry, numEntries)
		for i := range entries {
			entries[i] = push.Entry{Timestamp: time.Now(), Line: "level=info user=bob dur=1.5"}
		}

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(model.LabelSet{"app": "bench"}, entries...))

		runPipelineBenchmark(b, cfgs, []loki.Batch{batch})
	})

	b.Run("no entries match", func(b *testing.B) {
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"none\"}"

			stage.label_drop {
				values = ["unused"]
			}
		}
		`)

		entries := make([]push.Entry, numEntries)
		for i := range entries {
			entries[i] = push.Entry{Timestamp: time.Now(), Line: "level=info user=bob dur=1.5"}
		}

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(model.LabelSet{"app": "bench"}, entries...))

		runPipelineBenchmark(b, cfgs, []loki.Batch{batch})
	})

	b.Run("label matcher splits the batch", func(b *testing.B) {
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"bench\"}"

			stage.label_drop {
				values = ["unused"]
			}
		}
		`)

		// Split across two streams so the batch still holds numEntries in
		// total and stays comparable to the other cases.
		matching := make([]push.Entry, numEntries/2)
		for i := range matching {
			matching[i] = push.Entry{Timestamp: time.Now(), Line: "level=info user=bob dur=1.5"}
		}

		bypassed := make([]push.Entry, numEntries/2)
		for i := range bypassed {
			bypassed[i] = push.Entry{Timestamp: time.Now(), Line: "level=info user=bob dur=1.5"}
		}

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(model.LabelSet{"app": "bench"}, matching...))
		batch.Add(loki.NewStream(model.LabelSet{"app": "other"}, bypassed...))

		runPipelineBenchmark(b, cfgs, []loki.Batch{batch})
	})

	b.Run("line filter splits a stream", func(b *testing.B) {
		cfgs := loadConfig(`
		stage.match {
			selector = "{app=\"bench\"} |= \"match\""

			stage.label_drop {
				values = ["unused"]
			}
		}
		`)

		entries := make([]push.Entry, numEntries)
		for i := range entries {
			entries[i] = push.Entry{Timestamp: time.Now(), Line: "other level=info user=bob dur=1.5"}
			if i%2 == 0 {
				entries[i].Line = "match level=info user=bob dur=1.5"
			}
		}

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(model.LabelSet{"app": "bench"}, entries...))

		runPipelineBenchmark(b, cfgs, []loki.Batch{batch})
	})
}
