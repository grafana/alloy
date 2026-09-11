package stages

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

var (
	criTestTimeStr = "2019-01-01T01:00:00.000000001Z"
	criTestTime, _ = time.Parse(time.RFC3339Nano, criTestTimeStr)
	criTestTime2   = time.Now()

	tagFTime1Str = "2019-05-07T18:57:50.904275087+00:00"
	tagFTime1, _ = time.Parse(time.RFC3339Nano, tagFTime1Str)
	tagFTime2Str = "2019-05-07T18:57:55.904275087+00:00"
	tagFTime2, _ = time.Parse(time.RFC3339Nano, tagFTime2Str)
)

func TestCRIStage(t *testing.T) {
	type testCase struct {
		name                        string
		entries                     []Entry
		expected                    []Entry
		cfg                         CRIConfig
		expectedPartialLinesFlushed int
		expectedLinesTruncated      int
	}

	tests := []testCase{
		{
			name: "full line",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, criTestTimeStr+" stderr F message", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"flags": "F", "stream": "stderr", "content": "message", "time": criTestTimeStr},
					model.LabelSet{"stream": "stderr"},
					"message",
					criTestTime,
				),
			},
		},
		{
			name: "full line multiline",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, criTestTimeStr+" stderr F message\nmessage2", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"flags": "F", "stream": "stderr", "content": "message\nmessage2", "time": criTestTimeStr},
					model.LabelSet{"stream": "stderr"},
					"message\nmessage2",
					criTestTime,
				),
			},
		},
		{
			name: "with invalid timestamp",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "3242 stderr F message", criTestTime2),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"flags": "F", "stream": "stderr", "content": "message", "time": "3242"},
					model.LabelSet{"stream": "stderr"},
					"message",
					criTestTime2,
				),
			},
		},
		{
			name: "with invalid line",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm invalid!!!", criTestTime2),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm invalid!!!", criTestTime2),
			},
		},
		{
			name: "without flag",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "something stderr looks like it could be cri", criTestTime2),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"flags": "F", "stream": "stderr", "content": "looks like it could be cri", "time": "something"},
					model.LabelSet{"stream": "stderr"},
					"looks like it could be cri",
					criTestTime2,
				),
			},
		},
		{
			name: "tag F",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout F some full line", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "some full line", "time": tagFTime1Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"some full line",
					tagFTime1,
				),
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"log",
					tagFTime2,
				),
			},
		},
		{
			name: "tag P multi-stream",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime1Str+" stdout P partial line 2 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log finished", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime2Str+" stdout F another full log", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"partial line 1 log finished",
					tagFTime2,
				),
				newEntry(
					map[string]any{"foo": "bar2", "flags": "F", "stream": "stdout", "content": "another full log", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar2", "stream": "stdout"},
					"partial line 2 another full log",
					tagFTime2,
				),
			},
		},
		{
			name: "tag P single stream",
			cfg:  CRIConfig{MaxPartialLines: 3},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 2 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 3 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 4 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log finished", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F another full log", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"partial line 1 partial line 2 partial line 3 partial line 4 log finished",
					tagFTime2,
				),
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "another full log", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"another full log",
					tagFTime2,
				),
			},
		},
		{
			name: "tag P multi-stream with truncation",
			cfg:  CRIConfig{MaxPartialLines: 100, MaxPartialLineSizeTruncate: true, MaxPartialLineSize: 11},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime1Str+" stdout P partial", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log finished", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime2Str+" stdout F full", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"partial lin",
					tagFTime2,
				),
				newEntry(
					map[string]any{"foo": "bar2", "flags": "F", "stream": "stdout", "content": "full", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar2", "stream": "stdout"},
					"partialfull",
					tagFTime2,
				),
			},
			expectedLinesTruncated: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expectedMetrics := fmt.Sprintf(`
# HELP loki_process_cri_lines_truncated_total A count of lines that were truncated due to the max_partial_line_size limit
# TYPE loki_process_cri_lines_truncated_total counter
loki_process_cri_lines_truncated_total %d
# HELP loki_process_cri_partial_lines_flushed_total A count of partial lines that were flushed prematurely due to the max_partial_lines limit being exceeded
# TYPE loki_process_cri_partial_lines_flushed_total counter
loki_process_cri_partial_lines_flushed_total %d
`, tt.expectedLinesTruncated, tt.expectedPartialLinesFlushed)

			runPipelineTest(t, []StageConfig{{CRIConfig: &tt.cfg}}, tt.entries, tt.expected, expectedMetrics)
		})
	}
}

func TestCRIStageMaxPartialLinesExceeded(t *testing.T) {
	cfg := CRIConfig{MaxPartialLines: 3}
	cfgs := []StageConfig{{CRIConfig: &cfg}}

	newEntries := func() []Entry {
		entries := []Entry{
			newEntry(map[string]any{}, model.LabelSet{"label1": "val1", "label2": "val2"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
			newEntry(map[string]any{}, model.LabelSet{"label1": "val1"}, tagFTime1Str+" stdout P partial line 2 ", time.Now()),
			newEntry(map[string]any{}, model.LabelSet{"label1": "val1", "label2": "val2"}, tagFTime1Str+" stdout P partial line 3 ", time.Now()),
			newEntry(map[string]any{}, model.LabelSet{"label1": "val3"}, tagFTime1Str+" stdout P partial line 4 ", time.Now()),
			newEntry(map[string]any{}, model.LabelSet{"label1": "val4"}, tagFTime1Str+" stdout P partial line 5 ", time.Now()),
			newEntry(map[string]any{}, model.LabelSet{"label1": "val1", "label2": "val2"}, tagFTime2Str+" stdout F log finished", time.Now()),
			newEntry(map[string]any{}, model.LabelSet{"label1": "val3"}, tagFTime2Str+" stdout F another full log", time.Now()),
			newEntry(map[string]any{}, model.LabelSet{"label1": "val4"}, tagFTime2Str+" stdout F yet an another full log", time.Now()),
		}
		for i := range entries {
			for labelName, labelValue := range entries[i].Labels {
				entries[i].Extracted[string(labelName)] = string(labelValue)
			}
		}
		return entries
	}

	expectedMetrics := `
# HELP loki_process_cri_lines_truncated_total A count of lines that were truncated due to the max_partial_line_size limit
# TYPE loki_process_cri_lines_truncated_total counter
loki_process_cri_lines_truncated_total 0
# HELP loki_process_cri_partial_lines_flushed_total A count of partial lines that were flushed prematurely due to the max_partial_lines limit being exceeded
# TYPE loki_process_cri_partial_lines_flushed_total counter
loki_process_cri_partial_lines_flushed_total 3
`

	expected := []Entry{
		newEntry(
			map[string]any{"label1": "val1", "label2": "val2", "flags": "P", "stream": "stdout", "content": "partial line 3 ", "time": tagFTime1Str},
			model.LabelSet{"label1": "val1", "label2": "val2", "stream": "stdout"},
			"partial line 1 partial line 3 ",
			tagFTime1,
		),
		newEntry(
			map[string]any{"label1": "val1", "flags": "P", "stream": "stdout", "content": "partial line 2 ", "time": tagFTime1Str},
			model.LabelSet{"label1": "val1", "stream": "stdout"},
			"partial line 2 ",
			tagFTime1,
		),
		newEntry(
			map[string]any{"label1": "val3", "flags": "P", "stream": "stdout", "content": "partial line 4 ", "time": tagFTime1Str},
			model.LabelSet{"label1": "val3", "stream": "stdout"},
			"partial line 4 ",
			tagFTime1,
		),
		newEntry(
			map[string]any{"label1": "val1", "label2": "val2", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
			model.LabelSet{"label1": "val1", "label2": "val2", "stream": "stdout"},
			"log finished",
			tagFTime2,
		),
		newEntry(
			map[string]any{"label1": "val3", "flags": "F", "stream": "stdout", "content": "another full log", "time": tagFTime2Str},
			model.LabelSet{"label1": "val3", "stream": "stdout"},
			"another full log",
			tagFTime2,
		),
		newEntry(
			map[string]any{"label1": "val4", "flags": "F", "stream": "stdout", "content": "yet an another full log", "time": tagFTime2Str},
			model.LabelSet{"label1": "val4", "stream": "stdout"},
			"partial line 5 yet an another full log",
			tagFTime2,
		),
	}

	t.Run("Pipeline", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		p, err := NewPipeline(logging.NewSlogNop(), cfgs, registry, featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)

		out := p.Run(withInboundEntries(newEntries()...))
		var collected []Entry
		for e := range out {
			collected = append(collected, e)
		}

		assertEntriesUnordered(t, expected, collected)
		require.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expectedMetrics)))
	})

	t.Run("New Pipeline", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		var collected []Entry
		next := func(_ context.Context, entries []Entry) error {
			collected = append(collected, entries...)
			return nil
		}

		p, err := newPipeline(logging.NewSlogNop(), registry, featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(t, err)

		// One entry per call, matching how Stage.Run offers entries to the
		// limit check one at a time off the channel.
		for _, e := range newEntries() {
			require.NoError(t, p.process(context.Background(), []Entry{e}))
		}
		p.stop()

		assertEntriesUnordered(t, expected, collected)
		require.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expectedMetrics)))
	})
}

// TestCRIStageFlushOnShutdown verifies that buffered entries are flushed when stop is called.
// This is only implemented for the new pipeline.
func TestCRIStageFlushOnShutdown(t *testing.T) {
	var (
		partialTimeStr = "2019-05-07T18:57:50.904275087+00:00"
		partialTime, _ = time.Parse(time.RFC3339Nano, partialTimeStr)
	)

	var collected []Entry
	next := func(_ context.Context, entries []Entry) error {
		collected = append(collected, entries...)
		return nil
	}

	p, err := newPipeline(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, []StageConfig{{CRIConfig: &defaultCRIConfig}}, next)
	require.NoError(t, err)

	require.NoError(t, p.process(context.Background(), []Entry{
		newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, partialTimeStr+" stdout P partial line ", time.Now()),
	}))
	require.Empty(t, collected)

	p.stop()

	expected := []Entry{
		newEntry(
			map[string]any{"flags": "P", "stream": "stdout", "content": "partial line ", "time": partialTimeStr},
			model.LabelSet{"foo": "bar", "stream": "stdout"},
			"partial line ",
			partialTime,
		),
	}
	assertEntriesUnordered(t, expected, collected)
}

func TestPartialLinesStriped(t *testing.T) {
	now := time.Now()

	entry := func(line string) Entry {
		return newEntry(map[string]any{}, model.LabelSet{}, line, now)
	}

	counters := func() (truncated, flushed prometheus.Counter) {
		return prometheus.NewCounter(prometheus.CounterOpts{Name: "t"}), prometheus.NewCounter(prometheus.CounterOpts{Name: "f"})
	}

	t.Run("Complete without a prior Append returns the entry unchanged", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10, MaxPartialLineSize: 5, MaxPartialLineSizeTruncate: true}, logging.NewSlogNop(), truncated, flushed)

		got := pl.Complete(1, entry("a full line that never had a partial predecessor"))

		require.Equal(t, "a full line that never had a partial predecessor", got.Line)
		require.Zero(t, testutil.ToFloat64(truncated))
		require.Zero(t, pl.size.Load(), "a Complete that finds nothing buffered must not change the size")
	})

	t.Run("Append then Complete merges the accumulated partial lines", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("partial one "))
		require.Equal(t, int64(1), pl.size.Load())

		pl.Append(1, entry("partial two "))
		require.Equal(t, int64(1), pl.size.Load(), "merging into an existing stream must not grow the size")

		got := pl.Complete(1, entry("full line"))

		require.Equal(t, "partial one partial two full line", got.Line)
		require.Zero(t, pl.size.Load())
	})

	t.Run("Complete removes the entry so a repeated Complete does not see stale state", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("partial "))
		first := pl.Complete(1, entry("first full"))
		require.Equal(t, "partial first full", first.Line)

		require.Zero(t, pl.size.Load())

		second := pl.Complete(1, entry("second full"))
		require.Equal(t, "second full", second.Line)
		require.Zero(t, pl.size.Load(), "completing an already removed stream must not change the size")
	})

	t.Run("Append and Complete keep independent state per fingerprint", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("stream one "))
		pl.Append(2, entry("stream two "))
		require.Equal(t, int64(2), pl.size.Load())

		gotOne := pl.Complete(1, entry("full one"))
		gotTwo := pl.Complete(2, entry("full two"))

		require.Equal(t, "stream one full one", gotOne.Line)
		require.Equal(t, "stream two full two", gotTwo.Line)
		require.Zero(t, pl.size.Load())
	})

	t.Run("Append does not truncate when MaxPartialLineSizeTruncate is false", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10, MaxPartialLineSize: 5, MaxPartialLineSizeTruncate: false}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("abcdefg"))
		got := pl.Complete(1, entry("hij"))

		require.Equal(t, "abcdefghij", got.Line)
		require.Zero(t, testutil.ToFloat64(truncated))
	})

	t.Run("Append truncates the accumulated line once it reaches the configured max size", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10, MaxPartialLineSize: 5, MaxPartialLineSizeTruncate: true}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("abcdefg"))
		got := pl.Complete(1, entry("hij"))

		require.Equal(t, "abcde", got.Line)
		require.NotZero(t, testutil.ToFloat64(truncated))
	})

	t.Run("Append does not count a truncation when the new line discards nothing", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10, MaxPartialLineSize: 5, MaxPartialLineSizeTruncate: true}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("abcdefg"))
		require.Equal(t, float64(1), testutil.ToFloat64(truncated))

		// The buffer is already at max size and there is nothing to discard.
		pl.Append(1, entry(""))
		require.Equal(t, float64(1), testutil.ToFloat64(truncated))

		// A non-empty line is discarded, so this one does count.
		pl.Append(1, entry("h"))
		require.Equal(t, float64(2), testutil.ToFloat64(truncated))

		got := pl.Complete(1, entry(""))
		require.Equal(t, "abcde", got.Line)
	})

	t.Run("FlushAll drains every buffered entry and clears the state", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 10}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("one"))
		pl.Append(2, entry("two"))

		require.ElementsMatch(t, []Entry{entry("one"), entry("two")}, pl.FlushAll())
		require.Empty(t, pl.FlushAll())
		require.Zero(t, pl.size.Load())
	})

	t.Run("FlushIfExceeded below the threshold does nothing", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 3}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("one"))
		pl.Append(2, entry("two"))

		require.Nil(t, pl.FlushIfExceeded())
		require.Zero(t, testutil.ToFloat64(flushed))
		require.Equal(t, int64(2), pl.size.Load())
		require.ElementsMatch(t, []Entry{entry("one"), entry("two")}, pl.FlushAll())
	})

	t.Run("FlushIfExceeded at the threshold drains everything", func(t *testing.T) {
		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 2}, logging.NewSlogNop(), truncated, flushed)

		pl.Append(1, entry("one"))
		pl.Append(2, entry("two"))

		require.ElementsMatch(t, []Entry{entry("one"), entry("two")}, pl.FlushIfExceeded())
		require.Equal(t, float64(2), testutil.ToFloat64(flushed))
		require.Empty(t, pl.FlushAll())
		require.Zero(t, pl.size.Load())
	})
}

func TestPartialLinesStripedConcurrent(t *testing.T) {
	now := time.Now()

	counters := func() (truncated, flushed prometheus.Counter) {
		return prometheus.NewCounter(prometheus.CounterOpts{Name: "t"}), prometheus.NewCounter(prometheus.CounterOpts{Name: "f"})
	}

	streamEntry := func(g, i int, line string) Entry {
		labels := model.LabelSet{
			"pod":       model.LabelValue(fmt.Sprintf("pod-%d", g)),
			"container": model.LabelValue(fmt.Sprintf("container-%d", i)),
		}
		return newEntry(map[string]any{}, labels, line, now)
	}

	t.Run("workers do not mix up each others streams", func(t *testing.T) {
		const (
			workers    = 8
			partials   = 5
			iterations = 10
		)

		truncated, flushed := counters()
		pl := newPartialLinesStriped(
			CRIConfig{MaxPartialLines: workers * 2},
			logging.NewSlogNop(),
			truncated,
			flushed,
		)

		var (
			mut       sync.Mutex
			completed []Entry
		)

		var wg sync.WaitGroup
		for w := range workers {
			wg.Go(func() {
				for i := range iterations {
					for p := range partials {
						e := streamEntry(w, 0, fmt.Sprintf("w%d-i%d-p%d ", w, i, p))
						pl.Append(e.Labels.Fingerprint(), e)
					}

					full := streamEntry(w, 0, fmt.Sprintf("w%d-i%d-F", w, i))
					got := pl.Complete(full.Labels.Fingerprint(), full)

					mut.Lock()
					completed = append(completed, got)
					mut.Unlock()
				}
			})
		}
		wg.Wait()

		want := make(map[string]struct{}, workers*iterations)
		for w := range workers {
			for i := range iterations {
				var sb strings.Builder
				for p := range partials {
					fmt.Fprintf(&sb, "w%d-i%d-p%d ", w, i, p)
				}
				fmt.Fprintf(&sb, "w%d-i%d-F", w, i)
				want[sb.String()] = struct{}{}
			}
		}

		require.Len(t, completed, workers*iterations)
		for _, e := range completed {
			require.Contains(t, want, e.Line)
			delete(want, e.Line)
		}
		require.Empty(t, want)

		require.Zero(t, pl.size.Load())
		require.Zero(t, testutil.ToFloat64(truncated))
		require.Zero(t, testutil.ToFloat64(flushed))
	})

	t.Run("every partial line comes back out of a flush exactly once", func(t *testing.T) {
		const (
			workers          = 8
			streamsPerWorker = 8
			total            = workers * streamsPerWorker
		)

		truncated, flushed := counters()
		pl := newPartialLinesStriped(CRIConfig{MaxPartialLines: 4}, logging.NewSlogNop(), truncated, flushed)

		var (
			mut     sync.Mutex
			emitted []Entry
			// limitFlushed counts only the entries that left through
			// FlushIfExceeded. How many are still buffered when the workers
			// finish is timing dependent, so the total is not predictable.
			limitFlushed int
		)
		collectFlush := func(entries []Entry) {
			mut.Lock()
			defer mut.Unlock()
			emitted = append(emitted, entries...)
			limitFlushed += len(entries)
		}

		var wg sync.WaitGroup
		for w := range workers {
			wg.Go(func() {
				for st := range streamsPerWorker {
					e := streamEntry(w, st, fmt.Sprintf("w%d-s%d", w, st))
					collectFlush(pl.FlushIfExceeded())
					pl.Append(e.Labels.Fingerprint(), e)
				}
			})
		}
		wg.Wait()

		emitted = append(emitted, pl.FlushAll()...)

		seen := make(map[string]int, total)
		for _, e := range emitted {
			seen[e.Line]++
		}
		for w := range workers {
			for st := range streamsPerWorker {
				line := fmt.Sprintf("w%d-s%d", w, st)
				require.Equal(t, 1, seen[line], "%s must be emitted exactly once", line)
			}
		}

		require.Len(t, emitted, total)
		require.Zero(t, pl.size.Load())
		require.Empty(t, pl.FlushAll())
		// FlushAll is the shutdown drain and does not count towards the
		// metric, so only limit triggered flushes are counted.
		require.Equal(t, float64(limitFlushed), testutil.ToFloat64(flushed))
	})
}

func BenchmarkCRIStage(b *testing.B) {
	b.Run("single stream", func(b *testing.B) {
		batch := loki.NewBatch()
		batch.Add(loki.NewStream(model.LabelSet{}, push.Entry{
			Timestamp: time.Now(),
			Line:      "2019-01-01T01:00:00.000000001Z stderr F my cool message yay\n test",
		}))
		runPipelineBenchmark(b, []StageConfig{{CRIConfig: &defaultCRIConfig}}, []loki.Batch{batch})
	})

	b.Run("multiple streams", func(b *testing.B) {
		const (
			numBatches      = 10
			entriesPerBatch = 50
			partialPerGroup = 4
		)

		batches := make([]loki.Batch, numBatches)
		for i := range batches {
			labels := model.LabelSet{"worker": model.LabelValue(fmt.Sprintf("%d", i))}

			entries := make([]push.Entry, 0, entriesPerBatch)
			for len(entries) < entriesPerBatch {
				for p := 0; p < partialPerGroup; p++ {
					entries = append(entries, push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("2019-01-01T01:00:00.000000001Z stdout P part %d ", p),
					})
				}
				entries = append(entries, push.Entry{
					Timestamp: time.Now(),
					Line:      "2019-01-01T01:00:00.000000001Z stdout F end of line",
				})
			}

			batch := loki.NewBatch()
			batch.Add(loki.NewStream(labels, entries...))
			batches[i] = batch
		}

		runPipelineBenchmark(b, []StageConfig{{CRIConfig: &defaultCRIConfig}}, batches)
	})

	b.Run("flush pressure", func(b *testing.B) {
		const (
			numWorkers        = 10
			streamsPerWorker  = 5
			partialsPerStream = 3
			maxPartialLines   = 8
		)

		batches := make([]loki.Batch, numWorkers)
		for w := range batches {
			batch := loki.NewBatch()
			for s := 0; s < streamsPerWorker; s++ {
				labels := model.LabelSet{
					"worker": model.LabelValue(fmt.Sprintf("%d", w)),
					"stream": model.LabelValue(fmt.Sprintf("%d", s)),
				}

				for i := range partialsPerStream {
					flag, content := "P", fmt.Sprintf("partial %d ", i)
					if i == partialsPerStream-1 {
						flag, content = "F", "final line"
					}
					batch.AddEntry(labels, 0, push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("2019-01-01T01:00:00.000000001Z stdout %s %s", flag, content),
					})
				}
			}
			batches[w] = batch
		}

		runPipelineBenchmark(b, []StageConfig{{CRIConfig: &CRIConfig{MaxPartialLines: maxPartialLines}}}, batches)
	})
}
