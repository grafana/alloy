package stages

import (
	"context"
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

func TestPackPipeline(t *testing.T) {
	var (
		now = time.Now()
		cfg = `
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
		}
		`
	)
	entry1 := func() Entry {
		return newEntry(
			map[string]any{},
			model.LabelSet{
				"pod":       "foo-xsfs3",
				"container": "foo",
				"namespace": "dev",
				"cluster":   "us-eu-1",
			},
			testMatchLogLineApp1,
			now,
		)
	}
	entry2 := func() Entry {
		return newEntry(
			map[string]any{},
			model.LabelSet{
				"pod":       "foo-vvsdded",
				"container": "bar",
				"namespace": "dev",
				"cluster":   "us-eu-1",
			},
			regexLogFixture,
			now,
		)
	}

	assertPacked := func(t *testing.T, out1, out2 Entry) {
		// Expected labels should remove the packed labels
		expectedLbls := model.LabelSet{
			"namespace": "dev",
			"cluster":   "us-eu-1",
		}
		assert.Equal(t, expectedLbls, out1.Labels)
		assert.Equal(t, expectedLbls, out2.Labels)

		// Validate timestamps
		// Line 1 should use the first matcher and should use the log line timestamp
		assert.Equal(t, now, out1.Timestamp)
		// Line 2 should use the second matcher and should get timestamp by the pack stage
		assert.True(t, out2.Timestamp.After(now))

		// Unmarshal the packed object and validate line1
		w := &Packed{}
		assert.NoError(t, json.Unmarshal([]byte(out1.Entry.Entry.Line), w))
		assert.Equal(t, map[string]string{
			"pod":       "foo-xsfs3",
			"container": "foo",
		}, w.Labels)
		assert.Equal(t, testMatchLogLineApp1, w.Entry)

		// Validate line 2
		w = &Packed{}
		assert.NoError(t, json.Unmarshal([]byte(out2.Entry.Entry.Line), w))
		assert.Equal(t, map[string]string{
			"pod":       "foo-vvsdded",
			"container": "bar",
		}, w.Labels)
		assert.Equal(t, regexLogFixture, w.Entry)
	}

	t.Run("Pipeline", func(t *testing.T) {
		pl, err := NewPipeline(util.TestAlloyLogger(t).Slog(), loadConfig(cfg), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)

		out1 := processEntries(pl, entry1())[0]
		time.Sleep(1 * time.Millisecond)
		out2 := processEntries(pl, entry2())[0]

		assertPacked(t, out1, out2)
	})

	t.Run("New Pipeline", func(t *testing.T) {
		var collected []Entry
		next := func(_ context.Context, entries []Entry) error {
			collected = append(collected, entries...)
			return nil
		}

		p, err := newPipeline(util.TestAlloyLogger(t).Slog(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, loadConfig(cfg), next)
		require.NoError(t, err)

		require.NoError(t, p.process(context.Background(), []Entry{entry1()}))
		time.Sleep(1 * time.Millisecond)
		require.NoError(t, p.process(context.Background(), []Entry{entry2()}))
		p.stop()

		require.Len(t, collected, 2)
		assertPacked(t, collected[0], collected[1])
	})
}

func TestPackStage(t *testing.T) {
	type testCase struct {
		name     string
		cfg      string
		entries  []Entry
		expected []Entry
		check    entryCheckFNs
	}

	tests := []testCase{
		{
			name: "no supplied labels list",
			cfg: `
			stage.pack {
				labels           = []
				ingest_timestamp = false
			}
			`,
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
			cfg: `
			stage.pack {
				labels           = ["foo"]
				ingest_timestamp = false
			}
			`,
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
			cfg: `
			stage.pack {
				labels           = ["foo", "bar"]
				ingest_timestamp = false
			}
			`,
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
			cfg: `
			stage.pack {
				labels           = ["foo", "extr1"]
				ingest_timestamp = false
			}
			`,
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
			cfg: `
			stage.pack {
				labels           = ["foo", "extr2"]
				ingest_timestamp = false
			}
			`,
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
			cfg: `
			stage.pack {
				labels           = ["foo", "ex\"tr2"]
				ingest_timestamp = false
			}
			`,
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
			cfg: `
			stage.pack {
				labels           = []
				ingest_timestamp = true
			}
			`,
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

			runPipelineTest(t, loadConfig(tt.cfg), tt.entries, tt.expected, tt.check)
		})
	}
}
