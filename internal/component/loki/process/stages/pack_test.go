package stages

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	json "github.com/json-iterator/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
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

// TestDropPipeline is used to verify we properly parse the Alloy config and
// create a working pipeline.
func TestPackPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	plName := "test_pack_pipeline"
	logger := util.TestAlloyLogger(t)
	pl, err := NewPipeline(logger, loadConfig(testPackAlloy), &plName, registry, featuregate.StabilityGenerallyAvailable)
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
	out2 := processEntries(pl, newEntry(nil, l2Lbls, testRegexLogLine, testTime))[0]

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
	assert.Equal(t, testRegexLogLine, w.Entry)
}

func TestPackStage(t *testing.T) {
	tests := []struct {
		name          string
		config        *PackConfig
		inputEntry    Entry
		expectedEntry Entry
	}{
		{
			name: "no supplied labels list",
			config: &PackConfig{
				Labels:          nil,
				IngestTimestamp: false,
			},
			inputEntry: Entry{
				Extracted: map[string]any{},
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "test line 1",
					},
				},
			},
			expectedEntry: Entry{
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "{\"" + packedEntryKey + "\":\"test line 1\"}",
					},
				},
			},
		},
		{
			name: "match one supplied label",
			config: &PackConfig{
				Labels:          []string{"foo"},
				IngestTimestamp: false,
			},
			inputEntry: Entry{
				Extracted: map[string]any{},
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "test line 1",
					},
				},
			},
			expectedEntry: Entry{
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "{\"foo\":\"bar\",\"" + packedEntryKey + "\":\"test line 1\"}",
					},
				},
			},
		},
		{
			name: "match all supplied labels",
			config: &PackConfig{
				Labels:          []string{"foo", "bar"},
				IngestTimestamp: false,
			},
			inputEntry: Entry{
				Extracted: map[string]any{},
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "test line 1",
					},
				},
			},
			expectedEntry: Entry{
				Entry: loki.Entry{
					Labels: model.LabelSet{},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "{\"bar\":\"baz\",\"foo\":\"bar\",\"" + packedEntryKey + "\":\"test line 1\"}",
					},
				},
			},
		},
		{
			name: "match extracted map and labels",
			config: &PackConfig{
				Labels:          []string{"foo", "extr1"},
				IngestTimestamp: false,
			},
			inputEntry: Entry{
				Extracted: map[string]any{
					"extr1": "etr1val",
					"extr2": "etr2val",
				},
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "test line 1",
					},
				},
			},
			expectedEntry: Entry{
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "{\"extr1\":\"etr1val\",\"foo\":\"bar\",\"" + packedEntryKey + "\":\"test line 1\"}",
					},
				},
			},
		},
		{
			name: "extracted map value not convertable to a string",
			config: &PackConfig{
				Labels:          []string{"foo", "extr2"},
				IngestTimestamp: false,
			},
			inputEntry: Entry{
				Extracted: map[string]any{
					"extr1": "etr1val",
					"extr2": []int{1, 2, 3},
				},
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "test line 1",
					},
				},
			},
			expectedEntry: Entry{
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "{\"foo\":\"bar\",\"" + packedEntryKey + "\":\"test line 1\"}",
					},
				},
			},
		},
		{
			name: "escape quotes",
			config: &PackConfig{
				Labels:          []string{"foo", "ex\"tr2"},
				IngestTimestamp: false,
			},
			inputEntry: Entry{
				Extracted: map[string]any{
					"extr1":   "etr1val",
					"ex\"tr2": `"fd"`,
				},
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "test line 1",
					},
				},
			},
			expectedEntry: Entry{
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "{\"ex\\\"tr2\":\"\\\"fd\\\"\",\"foo\":\"bar\",\"" + packedEntryKey + "\":\"test line 1\"}",
					},
				},
			},
		},
		{
			name: "ingest timestamp",
			config: &PackConfig{
				Labels:          nil,
				IngestTimestamp: true,
			},
			inputEntry: Entry{
				Extracted: map[string]any{},
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0),
						Line:      "test line 1",
					},
				},
			},
			expectedEntry: Entry{
				Entry: loki.Entry{
					Labels: model.LabelSet{
						"foo": "bar",
						"bar": "baz",
					},
					Entry: push.Entry{
						Timestamp: time.Unix(1, 0), // Ignored in test execution below
						Line:      "{\"" + packedEntryKey + "\":\"test line 1\"}",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := util.TestAlloyLogger(t)
			m := newPackStage(logger, *tt.config, prometheus.DefaultRegisterer)
			// Normal pipeline operation will put all the labels into the extracted map
			// replicate that here.
			for labelName, labelValue := range tt.inputEntry.Labels {
				tt.inputEntry.Extracted[string(labelName)] = string(labelValue)
			}
			out := processEntries(m, tt.inputEntry)
			// Only verify the labels, line, and timestamp, this stage doesn't modify the extracted map
			// so there is no reason to verify it
			assert.Equal(t, tt.expectedEntry.Labels, out[0].Labels)
			assert.Equal(t, tt.expectedEntry.Line, out[0].Line)
			if tt.config.IngestTimestamp {
				assert.True(t, out[0].Timestamp.After(tt.inputEntry.Timestamp))
			} else {
				assert.Equal(t, tt.expectedEntry.Timestamp, out[0].Timestamp)
			}
		})
	}
}
