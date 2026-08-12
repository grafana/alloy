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

func TestMatchNestedLimitShutdown(t *testing.T) {
	assertPipelineStopsPromptly(t, testMatchNestedLimitAlloy)
}

// TestMatchNarrowFusionProcessorOnly checks that a keep match whose nested
// pipeline is entirely Processor-wrapped stages (static_labels here)
// qualifies for narrow fusion (syncNarrowFn gets set), and that its
// behavior for both a matching and a non-matching entry is unaffected.
//
// This goes through a Pipeline (not a bare matcherStage.Run) because
// that's the only path that actually uses trySyncNarrow/syncNarrowFn —
// matcherStage.Run is untouched and always uses the old channel-based
// runKeep/runDrop, regardless of whether syncNarrowFn is set. It's also
// the only path with a strict per-entry ordering guarantee: the old
// runKeep can reorder a matched entry (routed through an async nested
// pipeline) relative to an unmatched one (sent directly), which is exactly
// why a bare matcherStage.Run isn't the right thing to assert order
// against.
func TestMatchNarrowFusionProcessorOnly(t *testing.T) {
	s, err := newMatcherStage(util.TestAlloyLogger(t).Slog(), MatchConfig{
		Selector: `{app="loki"}`,
		Action:   MatchActionKeep,
		Stages: []StageConfig{
			{StaticLabelsConfig: &StaticLabelsConfig{Values: map[string]*string{"tag": ptrStr("matched")}}},
		},
	}, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)

	ms := s.(*matcherStage)
	require.NotNil(t, ms.syncNarrowFn, "nested pipeline is Processor-only, should qualify for narrow fusion")

	pl := &Pipeline{stages: []Stage{s}}
	out := processEntries(pl,
		newEntry(nil, model.LabelSet{"app": "loki"}, "line1", time.Now()),
		newEntry(nil, model.LabelSet{"app": "other"}, "line2", time.Now()),
	)
	require.Len(t, out, 2)
	assert.Equal(t, model.LabelValue("matched"), out[0].Labels["tag"])
	assert.NotContains(t, out[1].Labels, model.LabelName("tag"))
}

// TestMatchNarrowFusionHandRolledFallsBack checks that a keep match nesting
// even one hand-rolled stage (stage.json here, which is common in the
// motivating adaptive-logs use case) does NOT qualify for narrow fusion,
// and that it still behaves correctly via the existing channel-based path.
func TestMatchNarrowFusionHandRolledFallsBack(t *testing.T) {
	s, err := newMatcherStage(util.TestAlloyLogger(t).Slog(), MatchConfig{
		Selector: `{app="loki"}`,
		Action:   MatchActionKeep,
		Stages: []StageConfig{
			{JSONConfig: &JSONConfig{Expressions: map[string]string{"msg": ""}}},
		},
	}, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)

	ms := s.(*matcherStage)
	require.Nil(t, ms.syncNarrowFn, "nested pipeline has a hand-rolled stage, should not qualify")

	out := processEntries(s, newEntry(nil, model.LabelSet{"app": "loki"}, `{"msg":"hello"}`, time.Now()))
	require.Len(t, out, 1)
	assert.Equal(t, "hello", out[0].Extracted["msg"])
}

// TestMatchNarrowFusionDropAlwaysQualifies checks that a drop match always
// qualifies for narrow fusion (it never has a nested pipeline to disqualify
// it) and preserves drop semantics.
func TestMatchNarrowFusionDropAlwaysQualifies(t *testing.T) {
	s, err := newMatcherStage(util.TestAlloyLogger(t).Slog(), MatchConfig{
		Selector: `{app="loki"}`,
		Action:   MatchActionDrop,
	}, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)

	ms := s.(*matcherStage)
	fn, ok := ms.trySyncNarrow()
	require.True(t, ok)
	require.NotNil(t, fn)

	out := processEntries(s,
		newEntry(nil, model.LabelSet{"app": "loki"}, "dropped", time.Now()),
		newEntry(nil, model.LabelSet{"app": "other"}, "kept", time.Now()),
	)
	require.Len(t, out, 1)
	assert.Equal(t, "kept", out[0].Line)
}

// testSequentialReinjectionAlloy has two independent top-level match
// blocks, each with a Processor-only nested pipeline (so both should
// qualify for narrow fusion and get fused into the same goroutine by
// Pipeline.Run). The second match's selector only fires on a label the
// first match's nested pipeline sets — this is exactly the property a
// naive "shared drainer" fusion design would break: each match must see
// the PREVIOUS match's output, not the original entry, or match 2 would
// never fire.
var testSequentialReinjectionAlloy = `
stage.match {
	selector = "{app=\"loki\"}"
	action   = "keep"
	stage.static_labels {
		values = { stage1_ran = "true" }
	}
}
stage.match {
	selector = "{stage1_ran=\"true\"}"
	action   = "keep"
	stage.static_labels {
		values = { stage2_ran = "true" }
	}
}`

func TestMatchNarrowFusionSequentialReinjection(t *testing.T) {
	pl, err := newPipelineFromConfig(testSequentialReinjectionAlloy)
	require.NoError(t, err)

	out := processEntries(pl, newEntry(nil, model.LabelSet{"app": "loki"}, "line", time.Now()))
	require.Len(t, out, 1)
	assert.Equal(t, model.LabelValue("true"), out[0].Labels["stage1_ran"])
	assert.Equal(t, model.LabelValue("true"), out[0].Labels["stage2_ran"], "second match should have seen the first match's output, not the original entry")
}

// TestMatchNarrowFusionNestedMatch checks that narrow fusion propagates
// recursively: a keep match nesting another keep match, both with
// Processor-only content, should have the OUTER match's syncNarrowFn set
// too.
func TestMatchNarrowFusionNestedMatch(t *testing.T) {
	pl, err := newPipelineFromConfig(`
stage.match {
	selector = "{app=\"loki\"}"
	action   = "keep"
	stage.match {
		selector = "{app=\"loki\"}"
		action   = "keep"
		stage.static_labels {
			values = { inner_ran = "true" }
		}
	}
}`)
	require.NoError(t, err)
	require.Len(t, pl.stages, 1)
	outer, ok := pl.stages[0].(*matcherStage)
	require.True(t, ok)
	require.NotNil(t, outer.syncNarrowFn, "outer match's nested pipeline is itself a fully-qualifying match, should be fused")

	out := processEntries(pl, newEntry(nil, model.LabelSet{"app": "loki"}, "line", time.Now()))
	require.Len(t, out, 1)
	assert.Equal(t, model.LabelValue("true"), out[0].Labels["inner_ran"])
}

func ptrStr(s string) *string { return &s }
