package stages

import (
	"testing"
	"time"

	json "github.com/json-iterator/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
)

// Not all these are tested but are here to make sure the different types marshal without error
var testPackAlloy = `
stage.match {
		selector = "{container=\"foo\"}"
		stage.pack {
				labels           = ["pod", "container"]
				ingest_timestamp = false
		}
}
stage.match {
		selector = "{container=\"bar\"}"
		stage.pack {
				labels           = ["pod", "container"]
				ingest_timestamp = true
		}
}`

func TestPackPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	logger := util.TestAlloyLogger(t)
	pl, err := NewPipeline(logger.Slog(), loadConfig(testPackAlloy), registry, featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)

	l1Lbls := model.LabelSet{
		"pod":       "foo-xsfs3",
		"container": "foo",
		"namespace": "dev",
		"cluster":   "us-eu-1",
	}

	l2Lbls := model.LabelSet{
		"pod":       "foo-vvsdded",
		"container": "bar",
		"namespace": "dev",
		"cluster":   "us-eu-1",
	}

	testTime := time.Now()

	// Submit these both separately to get a deterministic output
	// Also, add a tiny delay so that the two entries don't end up with the
	// same timestamp due to the Windows' lower-resolution timers.
	out1 := processEntries(pl, newEntry(nil, l1Lbls, testMatchLogLineApp1, testTime))[0]
	time.Sleep(1 * time.Millisecond)
	out2 := processEntries(pl, newEntry(nil, l2Lbls, regexLogFixture, testTime))[0]

	// Expected labels should remove the packed labels
	expectedLbls := model.LabelSet{
		"namespace": "dev",
		"cluster":   "us-eu-1",
	}
	assert.Equal(t, expectedLbls, out1.Labels)
	assert.Equal(t, expectedLbls, out2.Labels)

	// Validate timestamps
	// Line 1 should use the first matcher and should use the log line timestamp
	assert.Equal(t, testTime, out1.Timestamp)
	// Line 2 should use the second matcher and should get timestamp by the pack stage
	assert.True(t, out2.Timestamp.After(testTime))

	// Unmarshal the packed object and validate line1
	w := &Packed{}
	assert.NoError(t, json.Unmarshal([]byte(out1.Entry.Entry.Line), w))
	expectedPackedLabels := map[string]string{
		"pod":       "foo-xsfs3",
		"container": "foo",
	}
	assert.Equal(t, expectedPackedLabels, w.Labels)
	assert.Equal(t, testMatchLogLineApp1, w.Entry)

	// Validate line 2
	w = &Packed{}
	assert.NoError(t, json.Unmarshal([]byte(out2.Entry.Entry.Line), w))
	expectedPackedLabels = map[string]string{
		"pod":       "foo-vvsdded",
		"container": "bar",
	}
	assert.Equal(t, expectedPackedLabels, w.Labels)
	assert.Equal(t, regexLogFixture, w.Entry)
}

func TestPackStage(t *testing.T) {
	type testCase struct {
		name     string
		cfg      PackConfig
		entries  []Entry
		expected []Entry
		check    entryCheckFNs
	}

	tests := []testCase{
		{
			name: "no supplied labels list",
			cfg: PackConfig{
				Labels:          nil,
				IngestTimestamp: false,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "test line 1", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"foo": "bar",
					"bar": "baz",
				}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "{\""+packedEntryKey+"\":\"test line 1\"}", time.Unix(1, 0)),
			},
		},
		{
			name: "match one supplied label",
			cfg: PackConfig{
				Labels:          []string{"foo"},
				IngestTimestamp: false,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "test line 1", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"foo": "bar",
					"bar": "baz",
				}, model.LabelSet{
					"bar": "baz",
				}, "{\"foo\":\"bar\",\""+packedEntryKey+"\":\"test line 1\"}", time.Unix(1, 0)),
			},
		},
		{
			name: "match all supplied labels",
			cfg: PackConfig{
				Labels:          []string{"foo", "bar"},
				IngestTimestamp: false,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "test line 1", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"foo": "bar",
					"bar": "baz",
				}, model.LabelSet{}, "{\"bar\":\"baz\",\"foo\":\"bar\",\""+packedEntryKey+"\":\"test line 1\"}", time.Unix(1, 0)),
			},
		},
		{
			name: "match extracted map and labels",
			cfg: PackConfig{
				Labels:          []string{"foo", "extr1"},
				IngestTimestamp: false,
			},
			entries: []Entry{
				newEntry(map[string]any{
					"extr1": "etr1val",
					"extr2": "etr2val",
				}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "test line 1", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"extr1": "etr1val",
					"extr2": "etr2val",
					"foo":   "bar",
					"bar":   "baz",
				}, model.LabelSet{
					"bar": "baz",
				}, "{\"extr1\":\"etr1val\",\"foo\":\"bar\",\""+packedEntryKey+"\":\"test line 1\"}", time.Unix(1, 0)),
			},
		},
		{
			name: "extracted map value not convertable to a string",
			cfg: PackConfig{
				Labels:          []string{"foo", "extr2"},
				IngestTimestamp: false,
			},
			entries: []Entry{
				newEntry(map[string]any{
					"extr1": "etr1val",
					"extr2": []int{1, 2, 3},
				}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "test line 1", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"extr1": "etr1val",
					"extr2": []int{1, 2, 3},
					"foo":   "bar",
					"bar":   "baz",
				}, model.LabelSet{
					"bar": "baz",
				}, "{\"foo\":\"bar\",\""+packedEntryKey+"\":\"test line 1\"}", time.Unix(1, 0)),
			},
		},
		{
			name: "escape quotes",
			cfg: PackConfig{
				Labels:          []string{"foo", "ex\"tr2"},
				IngestTimestamp: false,
			},
			entries: []Entry{
				newEntry(map[string]any{
					"extr1":   "etr1val",
					"ex\"tr2": `"fd"`,
				}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "test line 1", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"extr1":   "etr1val",
					"ex\"tr2": `"fd"`,
					"foo":     "bar",
					"bar":     "baz",
				}, model.LabelSet{
					"bar": "baz",
				}, "{\"ex\\\"tr2\":\"\\\"fd\\\"\",\"foo\":\"bar\",\""+packedEntryKey+"\":\"test line 1\"}", time.Unix(1, 0)),
			},
		},
		{
			name: "ingest timestamp",
			cfg: PackConfig{
				Labels:          nil,
				IngestTimestamp: true,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "test line 1", time.Unix(1, 0)),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"foo": "bar",
					"bar": "baz",
				}, model.LabelSet{
					"foo": "bar",
					"bar": "baz",
				}, "{\""+packedEntryKey+"\":\"test line 1\"}", time.Unix(1, 0)),
			},
			check: entryCheckFNs{
				timestamp: func(expected, actual time.Time) bool {
					return actual.After(expected)
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{PackConfig: &tt.cfg}}, tt.entries, tt.expected, "", tt.check)
		})
	}
}
