package stages

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax"
)

var (
	splitJSONTestTime    = time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	splitJSONTestCreated = int64(1714979289123456)
)

// newSplitJSONEntry populates the per-entry fields the stage must preserve,
// including the private created timestamp, which the stock newEntry helper
// leaves at its zero value and would therefore mask preservation bugs.
// (push.Entry.Parsed stays zero: it's loki-internal and unused in Alloy.)
func newSplitJSONEntry(line string, extracted map[string]any) Entry {
	if extracted == nil {
		extracted = map[string]any{"other": "keep"}
	}
	return Entry{
		Extracted: extracted,
		Entry: loki.NewEntryWithCreatedUnixMicro(
			model.LabelSet{"app": "test"},
			splitJSONTestCreated,
			push.Entry{
				Timestamp:          splitJSONTestTime,
				Line:               line,
				StructuredMetadata: push.LabelsAdapter{{Name: "trace_id", Value: "123"}},
			},
		),
	}
}

func TestSplitJSONStage_SplitsTopLevelArray(t *testing.T) {
	t.Parallel()

	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{})

	parent := newSplitJSONEntry(`[{"a":1},{"b":2}]`, nil)
	out := processEntries(stage, parent)

	require.Len(t, out, 2)
	assert.Equal(t, `{"a":1}`, out[0].Line)
	assert.Equal(t, `{"b":2}`, out[1].Line)
	for _, child := range out {
		assert.Equal(t, parent.Labels, child.Labels)
		assert.Equal(t, parent.Timestamp, child.Timestamp)
		assert.Equal(t, parent.StructuredMetadata, child.StructuredMetadata)
		assert.Equal(t, parent.Extracted, child.Extracted)
		assert.Equal(t, parent.Created(), child.Created())
	}
}

func TestSplitJSONStage_LineMode(t *testing.T) {
	t.Parallel()

	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{})

	tests := map[string]struct {
		line          string
		expectedLines []string
	}{
		"single element": {
			`[{"a":1}]`,
			[]string{`{"a":1}`},
		},
		"mixed element types": {
			`[1,"a",{"b":2},null,true,[2]]`,
			[]string{`1`, `"a"`, `{"b":2}`, `null`, `true`, `[2]`},
		},
		"empty array": {
			`[]`,
			nil,
		},
		"empty array with inner whitespace": {
			`[ ]`,
			nil,
		},
		"empty array with surrounding JSON whitespace": {
			" \t\r\n[] \t\r\n",
			nil,
		},
		"array with surrounding JSON whitespace": {
			" \t\r\n[1,2] \t\r\n",
			[]string{`1`, `2`},
		},
		"string element keeps its spelling": {
			`["a"]`,
			[]string{`"a"`},
		},
		"string escape spelling is kept": {
			`["\u00e9"]`,
			[]string{`"\u00e9"`},
		},
		"non-ASCII string is kept verbatim": {
			`["é"]`,
			[]string{`"é"`},
		},
		"large integer is not rounded": {
			`[9007199254740993]`,
			[]string{`9007199254740993`},
		},
		"number exponent is kept": {
			`[1.2300e+02]`,
			[]string{`1.2300e+02`},
		},
		"zero-leading exponent is accepted and kept": {
			`[0e1000]`,
			[]string{`0e1000`},
		},
		"inner element spacing is kept": {
			`[ { "a" : 1 } ]`,
			[]string{`{ "a" : 1 }`},
		},
	}

	for tName, tt := range tests {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()

			out := processEntries(stage, newSplitJSONEntry(tt.line, nil))

			require.Len(t, out, len(tt.expectedLines))
			for i, want := range tt.expectedLines {
				assert.Equal(t, want, out[i].Line)
			}
		})
	}
}

func TestSplitJSONStage_PassThrough(t *testing.T) {
	t.Parallel()

	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{})

	tests := map[string]string{
		"unterminated array":               `[{"a":1}`,
		"not json":                         `not json`,
		"invalid number literal":           `[-01]`,
		"invalid element":                  `[1, invalid]`,
		"trailing comma":                   `[1,]`,
		"trailing garbage":                 `[1] trailing`,
		"json object":                      `{"a":1}`,
		"json string":                      `"a"`,
		"json number":                      `1`,
		"json null":                        `null`,
		"empty line":                       "",
		"whitespace only":                  " \t\r\n",
		"non-json whitespace before array": "\u00a0[1,2]",
	}

	for tName, line := range tests {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()

			parent := newSplitJSONEntry(line, nil)
			out := processEntries(stage, parent)

			require.Len(t, out, 1)
			assert.Equal(t, parent, out[0])
		})
	}
}

func TestSplitJSONStage_SourceMode(t *testing.T) {
	t.Parallel()

	source := Source("records")
	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{Source: &source})

	extracted := map[string]any{
		"records": `[{"a":1},{"b":2}]`,
		"other":   "keep",
	}
	out := processEntries(stage, newSplitJSONEntry(`{"records":[{"a":1},{"b":2}]}`, extracted))

	require.Len(t, out, 2)
	assert.Equal(t, `{"a":1}`, out[0].Line)
	assert.Equal(t, `{"b":2}`, out[1].Line)
	for _, child := range out {
		assert.Equal(t, extracted, child.Extracted)
	}
}

func TestSplitJSONStage_SourceModePassThrough(t *testing.T) {
	t.Parallel()

	source := Source("records")
	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{Source: &source})

	tests := map[string]map[string]any{
		"source key absent":                   {"other": "keep"},
		"source value malformed":              {"records": `[{"a":1}`},
		"source value not an array":           {"records": `{"a":1}`},
		"source value not string convertible": {"records": map[string]any{"a": 1}},
	}

	for tName, extracted := range tests {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()

			// The line itself is a splittable array: proves the stage never
			// falls back to it when source is set.
			parent := newSplitJSONEntry(`[{"a":1},{"b":2}]`, extracted)
			out := processEntries(stage, parent)

			require.Len(t, out, 1)
			assert.Equal(t, parent, out[0])
		})
	}
}

func TestSplitJSONStage_SourceModeEmptyArray(t *testing.T) {
	t.Parallel()

	source := Source("records")
	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{Source: &source})

	out := processEntries(stage, newSplitJSONEntry(`[{"a":1}]`, map[string]any{"records": `[]`}))
	require.Empty(t, out)
}

// An empty source is rejected when the configuration is decoded, before any
// stage is constructed.
func TestSplitJSONStage_EmptySourceRejectedAtDecode(t *testing.T) {
	t.Parallel()

	var config Configs
	err := syntax.Unmarshal([]byte(`
stage.split_json {
    source = ""
}
`), &config)
	require.ErrorContains(t, err, "source cannot be empty")
}

func TestSplitJSONStage_ChildStateIsolation(t *testing.T) {
	t.Parallel()

	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{})

	// Three elements exercise both branches: cloned children (all but the
	// last) and the final child that reuses the original's allocations.
	parent := newSplitJSONEntry(`[{"a":1},{"b":2},{"c":3}]`, nil)
	out := processEntries(stage, parent)
	require.Len(t, out, 3)

	for _, child := range out {
		assert.Equal(t, splitJSONTestCreated, child.Created())
	}

	// Mutating both non-final children catches any aliasing between cloned
	// children as well as against the reused final child and the parent.
	for _, i := range []int{0, 1} {
		out[i].Labels["app"] = "mutated"
		out[i].Extracted["other"] = "mutated"
		out[i].StructuredMetadata[0].Value = "mutated"
	}

	for _, e := range []Entry{out[2], parent} {
		assert.Equal(t, model.LabelValue("test"), e.Labels["app"])
		assert.Equal(t, "keep", e.Extracted["other"])
		assert.Equal(t, "123", e.StructuredMetadata[0].Value)
	}
}

func TestSplitJSONStage_CrossParentOrdering(t *testing.T) {
	t.Parallel()

	stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{})

	out := processEntries(stage,
		newSplitJSONEntry(`[{"a":1},{"a":2}]`, nil),
		newSplitJSONEntry(`not an array`, nil),
		newSplitJSONEntry(`[]`, nil),
		newSplitJSONEntry(`[3,4]`, nil),
	)

	lines := make([]string, 0, len(out))
	for _, e := range out {
		lines = append(lines, e.Line)
	}
	assert.Equal(t, []string{`{"a":1}`, `{"a":2}`, `not an array`, `3`, `4`}, lines)
}

var testSplitJSONAlloy = `
stage.split_json {}
`

var testSplitJSONAlloyWithSource = `
stage.split_json {
    source = "x"
}
`

func TestSplitJSONPipeline(t *testing.T) {
	t.Parallel()

	stages := loadConfig(testSplitJSONAlloy)
	require.Len(t, stages, 1)
	require.NotNil(t, stages[0].SplitJSONConfig)
	assert.Nil(t, stages[0].SplitJSONConfig.Source)

	pl, err := NewPipeline(logging.NewSlogNop(), stages, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)
	out := processEntries(pl, newEntry(nil, nil, `[1,2]`, time.Now()))
	require.Len(t, out, 2)
	assert.Equal(t, `1`, out[0].Line)
	assert.Equal(t, `2`, out[1].Line)

	stages = loadConfig(testSplitJSONAlloyWithSource)
	require.Len(t, stages, 1)
	require.NotNil(t, stages[0].SplitJSONConfig)
	require.NotNil(t, stages[0].SplitJSONConfig.Source)
	assert.Equal(t, Source("x"), *stages[0].SplitJSONConfig.Source)

	pl, err = NewPipeline(logging.NewSlogNop(), stages, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)
	out = processEntries(pl, newEntry(map[string]any{"x": `[true,false]`}, nil, `not an array`, time.Now()))
	require.Len(t, out, 2)
	assert.Equal(t, `true`, out[0].Line)
	assert.Equal(t, `false`, out[1].Line)
}

var testSplitJSONAlloyContinuation = `
stage.json {
    expressions = { "records" = "" }
}
stage.split_json {
    source = "records"
}
stage.json {
    expressions = { "ts" = "" }
}
stage.timestamp {
    source = "ts"
    format = "RFC3339"
}
`

func TestSplitJSONPipeline_Continuation(t *testing.T) {
	t.Parallel()

	pl, err := newPipelineFromConfig(testSplitJSONAlloyContinuation)
	require.NoError(t, err)

	line := `{"records":[{"ts":"2024-05-06T07:00:00Z"},{"ts":"2024-05-06T08:00:00Z"}]}`
	out := processEntries(pl, newEntry(nil, nil, line, time.Now()))

	require.Len(t, out, 2)
	assert.Equal(t, `{"ts":"2024-05-06T07:00:00Z"}`, out[0].Line)
	assert.Equal(t, `{"ts":"2024-05-06T08:00:00Z"}`, out[1].Line)
	assert.Equal(t, time.Date(2024, 5, 6, 7, 0, 0, 0, time.UTC), out[0].Timestamp.UTC())
	assert.Equal(t, time.Date(2024, 5, 6, 8, 0, 0, 0, time.UTC), out[1].Timestamp.UTC())
}

// benchSplitJSONLine builds a top-level JSON array of n objects, each padded
// to exactly objectSize bytes.
func benchSplitJSONLine(n, objectSize int) string {
	elems := make([]string, n)
	for i := range elems {
		pad := objectSize - len(fmt.Sprintf(`{"id":%d,"payload":""}`, i))
		elems[i] = fmt.Sprintf(`{"id":%d,"payload":"%s"}`, i, strings.Repeat("a", pad))
	}
	return "[" + strings.Join(elems, ",") + "]"
}

func BenchmarkSplitJSONStage(b *testing.B) {
	benchmarks := []struct {
		name       string
		elements   int
		objectSize int
	}{
		{"10x1KiB", 10, 1024},
		{"100x120B", 100, 120},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			stage := newSplitJSONStage(logging.NewSlogNop(), SplitJSONConfig{})
			entry := newSplitJSONEntry(benchSplitJSONLine(bm.elements, bm.objectSize), nil)

			in := make(chan Entry)
			out := stage.Run(in)
			done := make(chan struct{})
			go func() {
				defer close(done)
				for range out {
				}
			}()

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				in <- entry
			}
			close(in)
			<-done
		})
	}
}
