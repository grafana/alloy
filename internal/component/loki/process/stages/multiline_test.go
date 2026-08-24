package stages

import (
	"context"
	"sync"
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

func TestMultilineStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "flush on new start line",
			config: `
			stage.multiline {
				firstline     = "^START"
				max_wait_time = "3s"
				trim_newlines = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line before 1", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line before 2", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 1", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 2", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation A", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation B", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "not a start line before 1", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "not a start line before 2", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 1\nnot a start line", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 2\ncontinuation A\ncontinuation B", now),
			},
		},
		{
			name: "multiple streams flush independently",
			config: `
			stage.multiline {
				firstline     = "^START"
				max_wait_time = "3s"
				trim_newlines = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"value": "one"}, "START line 1\r\n", now.Add(0*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "one"}, "not a start line 1\r\n", now.Add(1*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "two"}, "START line 1\n", now.Add(2*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "one"}, "not a start line 2\n", now.Add(3*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "two"}, "START line 2\n", now.Add(4*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "one"}, "START line 2", now.Add(5*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "one"}, "not a start line 1", now.Add(6*time.Millisecond)),
			},
			expected: []Entry{
				newEntry(map[string]any{"value": "one"}, model.LabelSet{"value": "one"}, "START line 1\nnot a start line 1\nnot a start line 2", now.Add(0*time.Millisecond)),
				newEntry(map[string]any{"value": "two"}, model.LabelSet{"value": "two"}, "START line 1", now.Add(2*time.Millisecond)),
				newEntry(map[string]any{"value": "two"}, model.LabelSet{"value": "two"}, "START line 2", now.Add(4*time.Millisecond)),
				newEntry(map[string]any{"value": "one"}, model.LabelSet{"value": "one"}, "START line 2\nnot a start line 1", now.Add(5*time.Millisecond)),
			},
		},
		{
			name: "max lines of 1",
			config: `
			stage.multiline {
				firstline     = "^START"
				max_lines     = 1
				max_wait_time = "1h"
				trim_newlines = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation line", now.Add(1*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 2", now.Add(2*time.Millisecond)),
			},
			expected: []Entry{
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "continuation line", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 2", now.Add(2*time.Millisecond)),
			},
		},
		{
			name: "start line entry preserved across repeated max_lines flushes",
			config: `
			stage.multiline {
				firstline     = "^START"
				max_lines     = 2
				max_wait_time = "3s"
				trim_newlines = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 1", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation 1", now.Add(1*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation 2", now.Add(2*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation 3", now.Add(3*time.Millisecond)),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation 4", now.Add(4*time.Millisecond)),
			},
			expected: []Entry{
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 1\ncontinuation 1", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "continuation 2\ncontinuation 3", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "continuation 4", now),
			},
		},
		{
			name: "structured metadata is kept per flushed block",
			config: `
			stage.multiline {
				firstline     = "^START"
				max_wait_time = "3s"
				trim_newlines = true
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"value": "one"}, push.Entry{
					Timestamp: now,
					Line:      "START line 1",
					StructuredMetadata: push.LabelsAdapter{
						push.LabelAdapter{Name: "sm-key1", Value: "sm-value1"},
					},
				}),
				newTestEntry(map[string]any{}, model.LabelSet{"value": "one"}, push.Entry{
					Timestamp: now.Add(1 * time.Millisecond),
					Line:      "START line 2",
					StructuredMetadata: push.LabelsAdapter{
						push.LabelAdapter{Name: "sm-key2", Value: "sm-value2"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"value": "one"}, model.LabelSet{"value": "one"}, push.Entry{
					Timestamp: now,
					Line:      "START line 1",
					StructuredMetadata: push.LabelsAdapter{
						push.LabelAdapter{Name: "sm-key1", Value: "sm-value1"},
					},
				}),
				newTestEntry(map[string]any{"value": "one"}, model.LabelSet{"value": "one"}, push.Entry{
					Timestamp: now.Add(1 * time.Millisecond),
					Line:      "START line 2",
					StructuredMetadata: push.LabelsAdapter{
						push.LabelAdapter{Name: "sm-key2", Value: "sm-value2"},
					},
				}),
			},
		},
		{
			name: "trim_newlines false leaves newlines in the joined lines",
			config: `
			stage.multiline {
				firstline     = "^START"
				max_wait_time = "3s"
				trim_newlines = false
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line before 1", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line before 2", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 1\n", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 2\r\n", now),
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 3", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "not a start line before 1", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "not a start line before 2", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 1\n\nnot a start line", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 2\r\n", now),
				newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 3", now),
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

func TestMultilineStageMaxWaitTime(t *testing.T) {
	cfgs := loadConfig(`
	stage.multiline {
		firstline     = "^START"
		max_wait_time = "100ms"
		trim_newlines = true
	}
	`)

	var (
		now     = time.Now()
		entries = []Entry{
			newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line", now),
			newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation line 1", now.Add(1*time.Second)),
			newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation line 2", now.Add(2*time.Second)),
			newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 2", now.Add(3*time.Second)),
			newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "continuation line 1", now.Add(4*time.Second)),
			newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "START line 3", now.Add(5*time.Second)),
			newEntry(map[string]any{}, model.LabelSet{"value": "active"}, "START active", now.Add(6*time.Second)),
			newEntry(map[string]any{}, model.LabelSet{"value": "idle"}, "START idle", now.Add(7*time.Second)),
			newEntry(map[string]any{}, model.LabelSet{"value": "active"}, "continuation active", now.Add(8*time.Second)),
		}
		expected = []Entry{
			newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line\ncontinuation line 1", entries[0].Timestamp),
			newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "continuation line 2", entries[2].Timestamp),
			newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 2", entries[3].Timestamp),
			newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "continuation line 1", entries[4].Timestamp),
			newEntry(map[string]any{"value": "label"}, model.LabelSet{"value": "label"}, "START line 3", entries[5].Timestamp),
			newEntry(map[string]any{"value": "idle"}, model.LabelSet{"value": "idle"}, "START idle", entries[7].Timestamp),
			newEntry(map[string]any{"value": "active"}, model.LabelSet{"value": "active"}, "START active\ncontinuation active", entries[6].Timestamp),
		}
	)

	t.Run("Stage", func(t *testing.T) {
		// Pipeline.Run seeds Extracted from Labels itself, so a plain clone is enough.
		cloned := cloneEntries(entries)

		pl, err := NewPipeline(logging.NewSlogNop(), cfgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)

		in := make(chan Entry, len(cloned))
		out := pl.Run(in)

		var (
			mu        sync.Mutex
			collected []Entry
			done      = make(chan struct{})
		)
		go func() {
			defer close(done)
			for e := range out {
				mu.Lock()
				collected = append(collected, e)
				mu.Unlock()
			}
		}()

		in <- cloned[0]
		in <- cloned[1]
		time.Sleep(300 * time.Millisecond)
		in <- cloned[2]
		in <- cloned[3]
		time.Sleep(300 * time.Millisecond)
		in <- cloned[4]
		in <- cloned[5]
		in <- cloned[6]
		in <- cloned[7]
		time.Sleep(50 * time.Millisecond)
		in <- cloned[8]

		close(in)
		<-done

		assertEntriesUnordered(t, expected, collected, entryCheckFNs{})
	})

	t.Run("New Stage", func(t *testing.T) {
		cloned := cloneEntries(entries)
		for i := range cloned {
			for labelName, labelValue := range cloned[i].Labels {
				cloned[i].Extracted[string(labelName)] = string(labelValue)
			}
		}

		var (
			mu        sync.Mutex
			collected []Entry
		)
		next := func(_ context.Context, entries []Entry) error {
			mu.Lock()
			collected = append(collected, entries...)
			mu.Unlock()
			return nil
		}

		p, err := NewPipeline2(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(t, err)
		defer p.Stop()

		require.NoError(t, p.process(context.Background(), []Entry{cloned[0]}))
		require.NoError(t, p.process(context.Background(), []Entry{cloned[1]}))
		time.Sleep(300 * time.Millisecond)
		require.NoError(t, p.process(context.Background(), []Entry{cloned[2]}))
		require.NoError(t, p.process(context.Background(), []Entry{cloned[3]}))
		time.Sleep(300 * time.Millisecond)
		require.NoError(t, p.process(context.Background(), []Entry{cloned[4]}))
		require.NoError(t, p.process(context.Background(), []Entry{cloned[5]}))
		require.NoError(t, p.process(context.Background(), []Entry{cloned[6]}))
		require.NoError(t, p.process(context.Background(), []Entry{cloned[7]}))
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, p.process(context.Background(), []Entry{cloned[8]}))

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			mu.Lock()
			defer mu.Unlock()
			assertEntriesUnordered(c, expected, collected, entryCheckFNs{})
		}, 2*time.Second, 100*time.Millisecond)
	})
}

// TestMultilineStageStreamsMapCleaned verifies that the streams map is empty after the stopping.
func TestMultilineStageStreamsMapCleanup(t *testing.T) {
	cfgs := loadConfig(`
	stage.multiline {
		firstline     = "^START"
		max_wait_time = "50ms"
		trim_newlines = true
	}
	`)

	t.Run("Stage", func(t *testing.T) {
		p, err := NewPipeline(logging.NewSlogNop(), cfgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)
		ms, ok := p.stages[0].(*multilineStage)
		require.True(t, ok)

		in := make(chan Entry, 2)
		out := p.Run(in)

		var (
			mu   sync.Mutex
			res  []Entry
			done = make(chan struct{})
		)
		go func() {
			defer close(done)
			for e := range out {
				mu.Lock()
				res = append(res, e)
				mu.Unlock()
			}
		}()

		in <- newEntry(map[string]any{}, model.LabelSet{"value": "stream-a"}, "START a", time.Now())
		in <- newEntry(map[string]any{}, model.LabelSet{"value": "stream-b"}, "START b", time.Now())
		in <- newEntry(map[string]any{}, model.LabelSet{"value": "stream-c"}, "START c", time.Now())
		close(in)
		<-done

		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(res) == 3
		}, 2*time.Second, 20*time.Millisecond)

		require.Equal(t, 0, len(ms.streams), "streams map should be empty after channel close")
	})

	t.Run("New Stage", func(t *testing.T) {
		var (
			mu  sync.Mutex
			res []Entry
		)
		next := func(_ context.Context, entries []Entry) error {
			mu.Lock()
			res = append(res, entries...)
			mu.Unlock()
			return nil
		}

		p, err := NewPipeline2(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(t, err)
		ms, ok := p.stages[0].(*multilineStage)
		require.True(t, ok)

		require.NoError(t, p.process(context.Background(), []Entry{
			newEntry(map[string]any{}, model.LabelSet{"value": "stream-a"}, "START a", time.Now()),
		}))

		require.NoError(t, p.process(context.Background(), []Entry{
			newEntry(map[string]any{}, model.LabelSet{"value": "stream-b"}, "START b", time.Now()),
		}))

		require.NoError(t, p.process(context.Background(), []Entry{
			newEntry(map[string]any{}, model.LabelSet{"value": "stream-c"}, "START c", time.Now()),
		}))

		p.Stop()

		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(res) == 3
		}, 2*time.Second, 20*time.Millisecond)

		require.Equal(t, 0, len(ms.streams), "streams map should be empty after Stop")
	})
}

// TestMultilineStagePassThroughNoLabelRace is a race-detector regression test.
// Pass-through entries (non-start lines before any start line) are emitted
// unchanged, meaning a downstream stage can share the same Labels map. A bug
// where FastFingerprint() was called after emitting an entry would race with
// downstream label mutations.
func TestMultilineStagePassThroughNoLabelRace(t *testing.T) {
	cfgs := loadConfig(`
	stage.multiline {
		firstline     = "^START"
		max_wait_time = "3s"
		trim_newlines = true
	}
	`)

	t.Run("Stage", func(t *testing.T) {
		pl, err := NewPipeline(logging.NewSlogNop(), cfgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)

		in := make(chan Entry)
		out := pl.Run(in)

		done := make(chan struct{})
		go func() {
			defer close(done)
			for e := range out {
				// Simulate a downstream stage (e.g. static_labels) mutating the
				// Labels map of a received entry. This races with any post-emit
				// read of e.Labels in the multiline goroutine.
				e.Labels["injected"] = "value"
			}
		}()

		go func() {
			for i := 0; i < 50; i++ {
				in <- newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line", time.Now())
			}
			close(in)
		}()

		<-done
	})

	t.Run("New Stage", func(t *testing.T) {
		var wg sync.WaitGroup
		next := func(_ context.Context, entries []Entry) error {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, e := range entries {
					// Simulate a downstream stage (e.g. static_labels)
					// mutating the Labels map of a received entry,
					// concurrently with this stage processing later batches.
					e.Labels["injected"] = "value"
				}
			}()
			return nil
		}

		p, err := NewPipeline2(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(t, err)

		for i := 0; i < 50; i++ {
			require.NoError(t, p.process(context.Background(), []Entry{
				newEntry(map[string]any{}, model.LabelSet{"value": "label"}, "not a start line", time.Now()),
			}))
		}

		wg.Wait()
		p.Stop()
	})
}

func TestValidateMultilineConfig(t *testing.T) {
	type testCase struct {
		name      string
		config    string
		expectErr bool
	}

	tests := []testCase{
		{
			name: "valid",
			config: `
				firstline     = "^START"
				max_wait_time = "3s"
			`,
		},
		{
			name: "max_wait_time must be greater than 0",
			config: `
				firstline     = "^START"
				max_wait_time = "0s"
			`,
			expectErr: true,
		},
		{
			name: "empty expression",
			config: `
				firstline = ""
			`,
			expectErr: true,
		},
		{
			name: "invalid regex",
			config: `
				firstline = "["
			`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg MultilineConfig
			err := syntax.Unmarshal([]byte(tt.config), &cfg)
			if err != nil {
				require.True(t, tt.expectErr, "unexpected error unmarshaling config: %v", err)
				return
			}

			_, err = validateMultilineConfig(cfg)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func BenchmarkMultilineStage(b *testing.B) {
	cfgs := loadConfig(`
	stage.multiline {
		firstline     = "^Date:"
		max_wait_time = "3s"
		max_lines     = 128
		trim_newlines = true
	}
	`)

	entries := make([]push.Entry, 10)
	entries[0] = push.Entry{Timestamp: time.Now(), Line: "Date: Mon, 01 Jan 2024 00:00:00 +0000 error occurred"}
	for i := 1; i < len(entries); i++ {
		entries[i] = push.Entry{Timestamp: time.Now(), Line: "\tat com.example.Foo.bar(Foo.java:42)"}
	}

	batch := loki.NewBatch()
	batch.Add(loki.NewStream(model.LabelSet{"job": "bench"}, entries...))

	runPipelineBenchmark(b, cfgs, batch)
}
