package stages

import (
	"context"
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

func TestSplitJSONStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "splits top level array into multiple entries, preserving labels, extracted map and structured metadata",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{"other": "keep"}, model.LabelSet{"app": "test"}, push.Entry{
					Timestamp:          now,
					Line:               `[{"a":1},{"b":2}]`,
					StructuredMetadata: push.LabelsAdapter{{Name: "trace_id", Value: "123"}},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"other": "keep", "app": "test"}, model.LabelSet{"app": "test"}, push.Entry{
					Timestamp:          now,
					Line:               `{"a":1}`,
					StructuredMetadata: push.LabelsAdapter{{Name: "trace_id", Value: "123"}},
				}),
				newTestEntry(map[string]any{"other": "keep", "app": "test"}, model.LabelSet{"app": "test"}, push.Entry{
					Timestamp:          now,
					Line:               `{"b":2}`,
					StructuredMetadata: push.LabelsAdapter{{Name: "trace_id", Value: "123"}},
				}),
			},
		},
		{
			name: "single element",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[{"a":1}]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"a":1}`, now),
			},
		},
		{
			name: "mixed element types",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[1,"a",{"b":2},null,true,[2]]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `1`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `"a"`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `{"b":2}`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `null`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `true`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[2]`, now),
			},
		},
		{
			name: "empty array",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[]`, now),
			},
			expected: []Entry{},
		},
		{
			name: "empty array with inner whitespace",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[ ]`, now),
			},
			expected: []Entry{},
		},
		{
			name: "empty array with surrounding JSON whitespace",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, " \t\r\n[] \t\r\n", now),
			},
			expected: []Entry{},
		},
		{
			name: "array with surrounding JSON whitespace",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, " \t\r\n[1,2] \t\r\n", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `1`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `2`, now),
			},
		},
		{
			name: "string element keeps its spelling",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `["a"]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `"a"`, now),
			},
		},
		{
			name: "string escape spelling is kept",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `["\u00e9"]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `"\u00e9"`, now),
			},
		},
		{
			name: "non-ASCII string is kept verbatim",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `["é"]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `"é"`, now),
			},
		},
		{
			name: "large integer is not rounded",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[9007199254740993]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `9007199254740993`, now),
			},
		},
		{
			name: "number exponent is kept",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[1.2300e+02]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `1.2300e+02`, now),
			},
		},
		{
			name: "zero-leading exponent is accepted and kept",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[0e1000]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `0e1000`, now),
			},
		},
		{
			name: "inner element spacing is kept",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[ { "a" : 1 } ]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{ "a" : 1 }`, now),
			},
		},
		{
			name: "not a top-level JSON array passes through unchanged",
			config: `
			stage.split_json {}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[{"a":1}`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `not json`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[-01]`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[1, invalid]`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[1,]`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[1] trailing`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `{"a":1}`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `"a"`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `1`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `null`, now),
				newEntry(map[string]any{}, model.LabelSet{}, ``, now),
				newEntry(map[string]any{}, model.LabelSet{}, " \t\r\n", now),
				newEntry(map[string]any{}, model.LabelSet{}, "\u00a0[1,2]", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `[{"a":1}`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `not json`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[-01]`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[1, invalid]`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[1,]`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `[1] trailing`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `{"a":1}`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `"a"`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `1`, now),
				newEntry(map[string]any{}, model.LabelSet{}, `null`, now),
				newEntry(map[string]any{}, model.LabelSet{}, ``, now),
				newEntry(map[string]any{}, model.LabelSet{}, " \t\r\n", now),
				newEntry(map[string]any{}, model.LabelSet{}, "\u00a0[1,2]", now),
			},
		},
		{
			name: "source mode splits array from extracted field",
			config: `
			stage.split_json {
				source = "records"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"records": `[{"a":1},{"b":2}]`,
					"other":   "keep",
				}, model.LabelSet{}, `{"records":[{"a":1},{"b":2}]}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"records": `[{"a":1},{"b":2}]`,
					"other":   "keep",
				}, model.LabelSet{}, `{"a":1}`, now),
				newEntry(map[string]any{
					"records": `[{"a":1},{"b":2}]`,
					"other":   "keep",
				}, model.LabelSet{}, `{"b":2}`, now),
			},
		},
		{
			name: "source mode passes through unchanged when source is missing, malformed, not an array, or not string-convertible",
			config: `
			stage.split_json {
				source = "records"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"other": "keep"}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
				newEntry(map[string]any{"records": `[{"a":1}`}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
				newEntry(map[string]any{"records": `{"a":1}`}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
				newEntry(map[string]any{"records": map[string]any{"a": 1}}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"other": "keep"}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
				newEntry(map[string]any{"records": `[{"a":1}`}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
				newEntry(map[string]any{"records": `{"a":1}`}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
				newEntry(map[string]any{"records": map[string]any{"a": 1}}, model.LabelSet{}, `[{"a":1},{"b":2}]`, now),
			},
		},
		{
			name: "source mode drops the entry when the source value is an empty array",
			config: `
			stage.split_json {
				source = "records"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"records": `[]`}, model.LabelSet{}, `[{"a":1}]`, now),
			},
			expected: []Entry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected)
		})
	}
}

func TestSplitJSONConfigAlloyUnmarshall(t *testing.T) {
	t.Parallel()

	var config Configs
	err := syntax.Unmarshal([]byte(`
		stage.split_json {
			source = ""
		}
	`), &config)
	require.ErrorContains(t, err, "source cannot be empty")
}

// TestSplitJSONStage_ChildStateIsolation verifies that mutating one of the
// children from a split doesn't affect the final unshared/reused child.
func TestSplitJSONStage_ChildStateIsolation(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cfgs := loadConfig(`stage.split_json {}`)

	newParent := func() Entry {
		return newTestEntry(map[string]any{"other": "keep"}, model.LabelSet{"app": "test"}, push.Entry{
			Timestamp:          now,
			Line:               `[{"a":1},{"b":2},{"c":3}]`,
			StructuredMetadata: push.LabelsAdapter{{Name: "trace_id", Value: "123"}},
		})
	}

	assertChildrenIsolated := func(t *testing.T, out []Entry) {
		require.Len(t, out, 3)

		// Mutating both non-final children catches aliasing between cloned
		// children as well as against the reused final child.
		for _, i := range []int{0, 1} {
			out[i].Labels["app"] = "mutated"
			out[i].Extracted["other"] = "mutated"
			out[i].StructuredMetadata[0].Value = "mutated"
		}

		last := out[2]
		assert.Equal(t, model.LabelValue("test"), last.Labels["app"])
		assert.Equal(t, "keep", last.Extracted["other"])
		assert.Equal(t, "123", last.StructuredMetadata[0].Value)
	}

	t.Run("Stage", func(t *testing.T) {
		p, err := NewPipeline(logging.NewSlogNop(), cfgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)

		out := p.Run(withInboundEntries(newParent()))
		var collected []Entry
		for e := range out {
			collected = append(collected, e)
		}

		assertChildrenIsolated(t, collected)
	})

	t.Run("New Stage", func(t *testing.T) {
		var collected []Entry
		next := func(_ context.Context, es []Entry) error {
			collected = append(collected, es...)
			return nil
		}

		p, err := NewPipeline2(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(t, err)
		require.NoError(t, p.process(context.Background(), []Entry{newParent()}))

		assertChildrenIsolated(t, collected)
	})
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
			cfg := SplitJSONConfig{}
			entry := newEntry(map[string]any{}, model.LabelSet{}, benchSplitJSONLine(bm.elements, bm.objectSize), time.Now())

			batch := loki.NewBatch()
			for range 10 {
				batch.Add(loki.NewStream(entry.Labels, entry.Entry.Entry))
			}

			runPipelineBenchmark(b, []StageConfig{{SplitJSONConfig: &cfg}}, batch)
		})
	}
}
