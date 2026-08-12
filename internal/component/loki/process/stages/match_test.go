package stages

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
)

var testMatchAlloy = `
stage.json {
		expressions = { "app" = "" }
}

stage.labels {
		values = { "app" = "" }
}

stage.match {
		selector = "{app=\"loki\"}"
		stage.json {
				expressions = { "msg" = "message" }
		}
		action = "keep"
}

stage.match {
		pipeline_name = "app2"
		selector = "{app=\"poki\"}"
		stage.json {
				expressions = { "msg" = "msg" }
		}
		action = "keep"
}

stage.output {
		source = "msg"
}
`

var testMatchLogLineApp1 = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN",
	"message" : "app1 log line"
}
`

var testMatchLogLineApp2 = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"poki",
	"component": ["parser","type"],
	"level" : "WARN",
	"msg" : "app2 log line"
}
`

func TestMatchStage(t *testing.T) {
	registry := prometheus.NewRegistry()
	logger := util.TestAlloyLogger(t)
	pl, err := NewPipeline(logger.Slog(), loadConfig(testMatchAlloy), registry, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}

	in := make(chan Entry)

	out := pl.Run(in)

	in <- newEntry(nil, nil, testMatchLogLineApp1, time.Now())

	e := <-out

	assert.Equal(t, "app1 log line", e.Line)

	// Process the second log line which should extract the output from the `msg` field
	e.Line = testMatchLogLineApp2
	e.Extracted = map[string]any{}
	in <- e
	e = <-out
	assert.Equal(t, "app2 log line", e.Line)
	close(in)
}

func TestMatcher(t *testing.T) {
	t.Parallel()
	tests := []struct {
		selector string
		labels   map[string]string
		action   string

		shouldDrop bool
		shouldRun  bool
		wantErr    bool
	}{
		{`{foo="bar"} |= "foo"`, map[string]string{"foo": "bar"}, MatchActionKeep, false, true, false},
		{`{foo="bar"} |~ "foo"`, map[string]string{"foo": "bar"}, MatchActionKeep, false, true, false},
		{`{foo="bar"} |= "bar"`, map[string]string{"foo": "bar"}, MatchActionKeep, false, false, false},
		{`{foo="bar"} |~ "bar"`, map[string]string{"foo": "bar"}, MatchActionKeep, false, false, false},
		{`{foo="bar"} != "bar"`, map[string]string{"foo": "bar"}, MatchActionKeep, false, true, false},
		{`{foo="bar"} !~ "bar"`, map[string]string{"foo": "bar"}, MatchActionKeep, false, true, false},
		{`{foo="bar"} != "foo"`, map[string]string{"foo": "bar"}, MatchActionKeep, false, false, false},
		{`{foo="bar"} |= "foo"`, map[string]string{"foo": "bar"}, MatchActionDrop, true, false, false},
		{`{foo="bar"} |~ "foo"`, map[string]string{"foo": "bar"}, MatchActionDrop, true, false, false},
		{`{foo="bar"} |= "bar"`, map[string]string{"foo": "bar"}, MatchActionDrop, false, false, false},
		{`{foo="bar"} |~ "bar"`, map[string]string{"foo": "bar"}, MatchActionDrop, false, false, false},
		{`{foo="bar"} != "bar"`, map[string]string{"foo": "bar"}, MatchActionDrop, true, false, false},
		{`{foo="bar"} !~ "bar"`, map[string]string{"foo": "bar"}, MatchActionDrop, true, false, false},
		{`{foo="bar"} != "foo"`, map[string]string{"foo": "bar"}, MatchActionDrop, false, false, false},
		{`{foo="bar"} !~ "[]"`, map[string]string{"foo": "bar"}, MatchActionDrop, false, false, true},
		{"foo", map[string]string{"foo": "bar"}, MatchActionKeep, false, false, true},
		{"{}", map[string]string{"foo": "bar"}, MatchActionKeep, false, false, true},
		{"{", map[string]string{"foo": "bar"}, MatchActionKeep, false, false, true},
		{"", map[string]string{"foo": "bar"}, MatchActionKeep, false, true, true},
		{`{foo="bar"}`, map[string]string{"foo": "bar"}, MatchActionKeep, false, true, false},
		{`{foo=""}`, map[string]string{"foo": "bar"}, MatchActionKeep, false, false, false},
		{`{foo=""}`, map[string]string{}, MatchActionKeep, false, true, false},
		{`{foo!="bar"}`, map[string]string{"foo": "bar"}, MatchActionKeep, false, false, false},
		{`{foo!="bar"}`, map[string]string{"foo": "bar"}, MatchActionDrop, false, false, false},
		{`{foo="bar",bar!="test"}`, map[string]string{"foo": "bar"}, MatchActionKeep, false, true, false},
		{`{foo="bar",bar!="test"}`, map[string]string{"foo": "bar"}, MatchActionDrop, true, false, false},
		{`{foo="bar",bar!="test"}`, map[string]string{"foo": "bar", "bar": "test"}, MatchActionKeep, false, false, false},
		{`{foo="bar",bar=~"te.*"}`, map[string]string{"foo": "bar", "bar": "test"}, MatchActionDrop, true, false, false},
		{`{foo="bar",bar=~"te.*"}`, map[string]string{"foo": "bar", "bar": "test"}, MatchActionKeep, false, true, false},
		{`{foo="bar",bar!~"te.*"}`, map[string]string{"foo": "bar", "bar": "test"}, MatchActionKeep, false, false, false},
		{`{foo="bar",bar!~"te.*"}`, map[string]string{"foo": "bar", "bar": "test"}, MatchActionDrop, false, false, false},

		{`{foo=""}`, map[string]string{}, MatchActionKeep, false, true, false},
		{`{foo="bar"} |= "foo" | status >= 200`, map[string]string{"foo": "bar"}, MatchActionKeep, false, false, true},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s/%s/%s", tt.selector, tt.labels, tt.action)

		t.Run(name, func(t *testing.T) {
			// Build a match config which has a simple label stage that when matched will add the test_label to
			// the labels in the pipeline.
			var stages []StageConfig
			if tt.action != MatchActionDrop {
				stages = []StageConfig{
					{
						LabelsConfig: &LabelsConfig{
							Values: map[string]*string{"test_label": nil},
						},
					},
				}
			}
			matchConfig := MatchConfig{
				tt.selector,
				stages,
				tt.action,
				"",
				"",
			}
			logger := util.TestAlloyLogger(t)
			s, err := newMatcherStage(logger.Slog(), matchConfig, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if (err != nil) != tt.wantErr {
				t.Errorf("withMatcher() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if s != nil {
				out := processEntries(s, newEntry(map[string]any{
					"test_label": "unimportant value",
				}, toLabelSet(tt.labels), "foo", time.Now()))

				if tt.shouldDrop {
					if len(out) != 0 {
						t.Errorf("stage should have been dropped but got %v", out)
					}
					return
				}
				// test_label should only be in the label set if the stage ran
				if _, ok := out[0].Labels["test_label"]; ok {
					if !tt.shouldRun {
						t.Error("stage ran but should have not")
					}
				}
			}
		})
	}
}

func TestValidateMatcherConfig(t *testing.T) {
	emptyStages := []StageConfig{}
	defaultStage := []StageConfig{{MatchConfig: &MatchConfig{}}}
	tests := []struct {
		name     string
		cfg      *MatchConfig
		wantErr  bool
		expected *MatchConfig
	}{
		{name: "pipeline name required", cfg: &MatchConfig{}, wantErr: true},
		{name: "selector required", cfg: &MatchConfig{Selector: ""}, wantErr: true},
		{name: "nil stages without dropping", cfg: &MatchConfig{PipelineName: "", Selector: `{app="foo"}`, Action: MatchActionKeep, Stages: nil}, wantErr: true},
		{name: "empty stages without dropping", cfg: &MatchConfig{Selector: `{app="foo"}`, Action: MatchActionKeep, Stages: emptyStages}, wantErr: true},
		{name: "stages with dropping", cfg: &MatchConfig{Selector: `{app="foo"}`, Action: MatchActionDrop, Stages: defaultStage}, wantErr: true},
		{name: "empty stages dropping", cfg: &MatchConfig{Selector: `{app="foo"}`, Action: MatchActionDrop, Stages: emptyStages}},
		{name: "stages without dropping", cfg: &MatchConfig{Selector: `{app="foo"}`, Action: MatchActionKeep, Stages: defaultStage}},
		{name: "bad selector", cfg: &MatchConfig{Selector: `{app="foo}`, Action: MatchActionKeep, Stages: defaultStage}, wantErr: true},
		{name: "bad action", cfg: &MatchConfig{Selector: `{app="foo}`, Action: "nope", Stages: emptyStages}, wantErr: true},
		{
			name:     "sets default action to keep",
			cfg:      &MatchConfig{Selector: `{app="foo"}`, Stages: defaultStage},
			wantErr:  false,
			expected: &MatchConfig{Selector: `{app="foo"}`, Action: MatchActionKeep, Stages: defaultStage},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateMatcherConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMatcherConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.expected != nil {
				require.Equal(t, tt.expected, tt.cfg)
			}
		})
	}
}

func TestMatchStage_NewPipelineErrorIsWrapped(t *testing.T) {
	cfg := StageConfig{
		MatchConfig: &MatchConfig{
			Selector: `{app="loki"}`,
			Action:   MatchActionKeep,
			Stages: []StageConfig{
				{RegexConfig: &RegexConfig{Expression: "[unclosed"}},
			},
		},
	}

	logger := util.TestAlloyLogger(t)
	_, err := New(logger.Slog(), cfg, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
	require.ErrorContains(t, err, "match stage failed to create pipeline")
	require.ErrorContains(t, errors.Unwrap(err), "invalid stage config")
}

var testMatchNestedLimitAlloy = `
stage.match {
		selector = "{app=\"loki\"}"
		action = "keep"
		stage.limit {
				rate  = 0.1
				burst = 1
				drop  = false
		}
}`

// testDoubleNestedAsyncMatchAlloy nests a match inside a match, with a
// multiline stage at the innermost level. A multiline stage forces its own
// pipeline to become an asyncMatchStage (see newMatcherStage), and that in
// turn forces every match wrapping it to become an asyncMatchStage too, all
// the way up. This exercises that propagation two levels deep instead of
// just one.
var testDoubleNestedAsyncMatchAlloy = `
stage.match {
		selector = "{app=\"loki\"}"
		action   = "keep"
		stage.match {
				selector = "{app=\"loki\"}"
				action   = "keep"
				stage.multiline {
						firstline     = "^NEVER_MATCHES"
						max_wait_time = "3s"
				}
				stage.static_labels {
						values = { inner_ran = "true" }
				}
		}
}`

func TestNestedAsyncMatch(t *testing.T) {
	pl, err := newPipelineFromConfig(testDoubleNestedAsyncMatchAlloy)
	require.NoError(t, err)

	out := processEntries(pl, newEntry(nil, model.LabelSet{"app": "loki"}, "plain line", time.Now()))
	require.Len(t, out, 1)
	assert.Equal(t, model.LabelValue("true"), out[0].Labels["inner_ran"])
}

// testMatchNestedLimitWithOuterStagesAlloy sandwiches a keep-match around a
// blocking stage.limit between a sync stage before and after it, so a
// single entry must pass through: fused sync stage -> asyncMatchStage
// channel boundary -> fused sync stage.
var testMatchNestedLimitWithOuterStagesAlloy = `
stage.static_labels {
		values = { before = "yes" }
}
stage.match {
		selector = "{app=\"loki\"}"
		action   = "keep"
		stage.limit {
				rate  = 100
				burst = 100
				drop  = false
		}
}
stage.static_labels {
		values = { after = "yes" }
}`

func TestMatchNestedLimitWithOuterStages(t *testing.T) {
	pl, err := newPipelineFromConfig(testMatchNestedLimitWithOuterStagesAlloy)
	require.NoError(t, err)

	out := processEntries(pl, newEntry(nil, model.LabelSet{"app": "loki"}, "line", time.Now()))
	require.Len(t, out, 1)
	assert.Equal(t, model.LabelValue("yes"), out[0].Labels["before"])
	assert.Equal(t, model.LabelValue("yes"), out[0].Labels["after"])
}

// testMatchNestedSplitJSONAlloy nests a fan-out stage (split_json, a
// ChannelStage) inside a keep-match, forcing that match to become an
// asyncMatchStage. This checks the fan-out is relayed out of the match
// boundary intact, rather than only ever being tested at the top level.
var testMatchNestedSplitJSONAlloy = `
stage.match {
		selector = "{app=\"loki\"}"
		action   = "keep"
		stage.split_json {}
}`

func TestMatchNestedSplitJSON(t *testing.T) {
	pl, err := newPipelineFromConfig(testMatchNestedSplitJSONAlloy)
	require.NoError(t, err)

	out := processEntries(pl, newEntry(nil, model.LabelSet{"app": "loki"}, `[{"a":1},{"b":2},{"c":3}]`, time.Now()))
	require.Len(t, out, 3)
	assert.Equal(t, `{"a":1}`, out[0].Line)
	assert.Equal(t, `{"b":2}`, out[1].Line)
	assert.Equal(t, `{"c":3}`, out[2].Line)
}

func TestMatchNestedLimitShutdown(t *testing.T) {
	assertPipelineStopsPromptly(t, testMatchNestedLimitAlloy)
}
