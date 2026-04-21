package stages

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
)

func TestMultilineStageProcess(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	out := processEntries(stage,
		simpleEntry("not a start line before 1", "label"),
		simpleEntry("not a start line before 2", "label"),
		simpleEntry("START line 1", "label"),
		simpleEntry("not a start line", "label"),
		simpleEntry("START line 2", "label"),
		simpleEntry("START line 3", "label"))

	require.Len(t, out, 5)
	require.Equal(t, "not a start line before 1", out[0].Line)
	require.Equal(t, "not a start line before 2", out[1].Line)
	require.Equal(t, "START line 1\nnot a start line", out[2].Line)
	require.Equal(t, "START line 2", out[3].Line)
	require.Equal(t, "START line 3", out[4].Line)
}

func TestMultilineStageMultiStreams(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	out := processEntries(stage,
		simpleEntry("START line 1\r\n", "one"),
		simpleEntry("not a start line 1\r\n", "one"),
		simpleEntry("START line 1\n", "two"),
		simpleEntry("not a start line 2\n", "one"),
		simpleEntry("START line 2\n", "two"),
		simpleEntry("START line 2", "one"),
		simpleEntry("not a start line 1", "one"),
	)

	sort.Slice(out, func(l, r int) bool {
		return out[l].Timestamp.Before(out[r].Timestamp)
	})

	require.Len(t, out, 4)

	require.Equal(t, "START line 1\nnot a start line 1\nnot a start line 2", out[0].Line)
	require.Equal(t, model.LabelValue("one"), out[0].Labels["value"])

	require.Equal(t, "START line 1", out[1].Line)
	require.Equal(t, model.LabelValue("two"), out[1].Labels["value"])

	require.Equal(t, "START line 2", out[2].Line)
	require.Equal(t, model.LabelValue("two"), out[2].Labels["value"])

	require.Equal(t, "START line 2\nnot a start line 1", out[3].Line)
	require.Equal(t, model.LabelValue("one"), out[3].Labels["value"])
}

func TestMultilineStageProcessLeaveNewlines(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second, TrimNewlines: false}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	out := processEntries(stage,
		simpleEntry("not a start line before 1", "label"),
		simpleEntry("not a start line before 2", "label"),
		simpleEntry("START line 1\n", "label"),
		simpleEntry("not a start line", "label"),
		simpleEntry("START line 2\r\n", "label"),
		simpleEntry("START line 3", "label"))

	require.Len(t, out, 5)
	require.Equal(t, "not a start line before 1", out[0].Line)
	require.Equal(t, "not a start line before 2", out[1].Line)
	require.Equal(t, "START line 1\n\nnot a start line", out[2].Line)
	require.Equal(t, "START line 2\r\n", out[3].Line)
	require.Equal(t, "START line 3", out[4].Line)
}

func TestMultilineStageMaxWaitTime(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 100 * time.Millisecond, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	in := make(chan Entry, 2)
	out := stage.Run(in)

	// Accumulate result
	mu := new(sync.Mutex)
	var res []Entry
	go func() {
		for e := range out {
			mu.Lock()
			t.Logf("appending %s", e.Line)
			res = append(res, e)
			mu.Unlock()
		}
	}()

	// Write input with a delay
	go func() {
		in <- simpleEntry("START line", "label")

		// Trigger flush due to max wait timeout
		time.Sleep(150 * time.Millisecond)

		in <- simpleEntry("not a start line hitting timeout", "label")

		// Signal pipeline we are done.
		close(in)
	}()

	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(res) == 2 }, 2*time.Second, 200*time.Millisecond)
	require.Equal(t, "START line", res[0].Line)
	require.Equal(t, "not a start line hitting timeout", res[1].Line)
}

func TestMultilineStageStartLineFlushedBeforeNew(t *testing.T) {
	mcfg := MultilineConfig{
		Expression:   "^START",
		MaxLines:     2,
		MaxWaitTime:  3 * time.Second,
		TrimNewlines: true,
	}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	startTs := time.Now()
	lset := model.LabelSet{"value": "label"}

	out := processEntries(stage,
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs, Line: "START line 1"}),
		},
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs.Add(1 * time.Second), Line: "continuation line 1"}),
		},
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs.Add(2 * time.Second), Line: "continuation line 2"}),
		},
	)

	require.Len(t, out, 2)
	require.Equal(t, lset, out[0].Labels)
	require.Equal(t, startTs, out[0].Timestamp)
	require.Equal(t, "START line 1\ncontinuation line 1", out[0].Line)

	require.Equal(t, lset, out[1].Labels)
	require.Equal(t, startTs, out[1].Timestamp)
	require.Equal(t, "continuation line 2", out[1].Line)
}

// TestMultilineStageMultipleMaxLinesFlushes verifies that startLineEntry is
// preserved across max_lines flushes, so all sub-blocks inherit the original
// start line's timestamp.
func TestMultilineStageMultipleMaxLinesFlushes(t *testing.T) {
	mcfg := MultilineConfig{
		Expression:   "^START",
		MaxLines:     2,
		MaxWaitTime:  3 * time.Second,
		TrimNewlines: true,
	}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	startTs := time.Now()
	lset := model.LabelSet{"value": "label"}

	out := processEntries(stage,
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs, Line: "START line 1"}),
		},
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs.Add(1 * time.Second), Line: "continuation 1"}),
		},
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs.Add(2 * time.Second), Line: "continuation 2"}),
		},
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs.Add(3 * time.Second), Line: "continuation 3"}),
		},
		Entry{
			Extracted: map[string]any{},
			Entry:     loki.NewEntry(lset.Clone(), push.Entry{Timestamp: startTs.Add(4 * time.Second), Line: "continuation 4"}),
		},
	)

	require.Len(t, out, 3)
	require.Equal(t, "START line 1\ncontinuation 1", out[0].Line)
	require.Equal(t, startTs, out[0].Timestamp)
	require.Equal(t, "continuation 2\ncontinuation 3", out[1].Line)
	require.Equal(t, startTs, out[1].Timestamp)
	require.Equal(t, "continuation 4", out[2].Line)
	require.Equal(t, startTs, out[2].Timestamp)
}

func TestMultilineStageKeepingStructuredMetadata(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	line1 := Entry{
		Extracted: map[string]any{},
		Entry: loki.Entry{
			Labels: model.LabelSet{"value": "one"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "START line 1",
				StructuredMetadata: push.LabelsAdapter{
					push.LabelAdapter{
						Name:  "sm-key1",
						Value: "sm-value1",
					},
				},
			},
		},
	}
	time.Sleep(1 * time.Millisecond)
	line2 := Entry{
		Extracted: map[string]any{},
		Entry: loki.Entry{
			Labels: model.LabelSet{"value": "one"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "START line 2",
				StructuredMetadata: push.LabelsAdapter{
					push.LabelAdapter{
						Name:  "sm-key2",
						Value: "sm-value2",
					},
				},
			},
		},
	}

	out := processEntries(stage,
		line1,
		line2,
	)

	sort.Slice(out, func(l, r int) bool {
		return out[l].Timestamp.Before(out[r].Timestamp)
	})

	require.Len(t, out, 2)

	require.Equal(t, "START line 1", out[0].Line)
	require.Equal(t, model.LabelValue("one"), out[0].Labels["value"])
	require.Equal(t, "sm-key1", out[0].StructuredMetadata[0].Name)
	require.Equal(t, "sm-value1", out[0].StructuredMetadata[0].Value)

	require.Equal(t, "START line 2", out[1].Line)
	require.Equal(t, model.LabelValue("one"), out[1].Labels["value"])
	require.Equal(t, "sm-key2", out[1].StructuredMetadata[0].Name)
	require.Equal(t, "sm-value2", out[1].StructuredMetadata[0].Value)
}

// TestMultilineStageMaxWaitTimeMultiStream verifies that the timeout flush only
// affects streams that have been idle for at least max_wait_time, leaving
// recently-active streams alone.
func TestMultilineStageMaxWaitTimeMultiStream(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 100 * time.Millisecond, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	in := make(chan Entry, 4)
	out := stage.Run(in)

	mu := new(sync.Mutex)
	var res []Entry
	go func() {
		for e := range out {
			mu.Lock()
			res = append(res, e)
			mu.Unlock()
		}
	}()

	go func() {
		// Stream "idle" accumulates a block and then goes quiet — should be
		// flushed by the timeout.
		in <- simpleEntry("START idle", "idle")

		// Stream "active" starts a block around the same time.
		in <- simpleEntry("START active", "active")

		// After the timeout window, "active" gets a new line — keeping it alive.
		time.Sleep(80 * time.Millisecond)
		in <- simpleEntry("continuation active", "active")

		// Wait long enough for the idle stream to time out while active does not.
		time.Sleep(150 * time.Millisecond)

		close(in)
	}()

	// Wait for both streams to eventually flush (timeout + close).
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(res) == 2
	}, 2*time.Second, 20*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	lines := map[string]bool{}
	for _, e := range res {
		lines[e.Line] = true
	}
	require.True(t, lines["START idle"], "idle stream should have been flushed by timeout")
	require.True(t, lines["START active\ncontinuation active"], "active stream should have been flushed on close")
}

// TestMultilineStagePostTimeoutContinuation verifies the three-phase sequence:
// (1) start line accumulated, (2) timeout flushes the block, (3) a non-start
// line arrives and is accumulated as a new block using the preserved
// startLineEntry, (4) a new start line flushes that block.
func TestMultilineStagePostTimeoutContinuation(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 100 * time.Millisecond, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{
		cfg:    mcfg,
		regex:  regex,
		logger: log.NewNopLogger(),
	}

	in := make(chan Entry, 4)
	out := stage.Run(in)

	mu := new(sync.Mutex)
	var res []Entry
	go func() {
		for e := range out {
			mu.Lock()
			res = append(res, e)
			mu.Unlock()
		}
	}()

	go func() {
		in <- simpleEntry("START first", "label")

		// Let the timeout flush the first block.
		time.Sleep(150 * time.Millisecond)

		// Non-start line after timeout — accumulated as a new block.
		in <- simpleEntry("continuation after timeout", "label")

		// New start line should flush the accumulated non-start block, then
		// begin its own block.
		in <- simpleEntry("START second", "label")

		close(in)
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(res) == 3
	}, 2*time.Second, 20*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	require.Equal(t, "START first", res[0].Line)
	require.Equal(t, "continuation after timeout", res[1].Line)
	require.Equal(t, "START second", res[2].Line)
}

// TestMultilineStageStreamsMapCleanedUpOnClose verifies that the streams map
// is empty after the input channel closes, so that closed-over state from one
// pipeline run cannot leak into a future run.
func TestMultilineStageStreamsMapCleanedUpOnClose(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)
	stage := &multilineStage{
		cfg:     mcfg,
		regex:   regex,
		logger:  log.NewNopLogger(),
		streams: make(map[model.Fingerprint]*multilineState),
	}

	processEntries(stage,
		simpleEntry("START a", "stream-a"),
		simpleEntry("START b", "stream-b"),
		simpleEntry("START c", "stream-c"),
	)

	require.Equal(t, 0, len(stage.streams), "streams map should be empty after channel close")
}

// TestMultilineStageStreamsMapCleanedUpAfterTimeout verifies that streams are
// removed from the map when the timer-based flush fires, so the map does not
// accumulate dead entries for streams that go idle.
func TestMultilineStageStreamsMapCleanedUpAfterTimeout(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 50 * time.Millisecond, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)
	stage := &multilineStage{
		cfg:     mcfg,
		regex:   regex,
		logger:  log.NewNopLogger(),
		streams: make(map[model.Fingerprint]*multilineState),
	}

	in := make(chan Entry, 3)
	in <- simpleEntry("START a", "stream-a")
	in <- simpleEntry("START b", "stream-b")
	in <- simpleEntry("START c", "stream-c")

	out := stage.Run(in)

	mu := new(sync.Mutex)
	var res []Entry
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range out {
			mu.Lock()
			res = append(res, e)
			mu.Unlock()
		}
	}()

	// Wait until the timer has flushed all 3 streams. Verifying len(res)==3
	// before closing in proves the timer (not the close path) did the flush,
	// which also deleted the streams from the map in the same goroutine.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(res) == 3
	}, 2*time.Second, 20*time.Millisecond)

	// Close input and wait for the Run goroutine to exit; m.streams is then
	// safe to inspect without a data race.
	close(in)
	<-done

	require.Equal(t, 0, len(stage.streams), "streams map should be empty after timer-based flush")
}

// TestMultilineStagePassThroughNoLabelRace is a race-detector regression test.
// Pass-through entries (non-start lines before any start line) are emitted
// unchanged, meaning the downstream stage goroutine shares the same Labels map.
// A bug where FastFingerprint() was called after out<-r would race with
// downstream label mutations; this test exercises that path.
func TestMultilineStagePassThroughNoLabelRace(t *testing.T) {
	mcfg := MultilineConfig{Expression: "^START", MaxWaitTime: 3 * time.Second, TrimNewlines: true}
	regex, err := validateMultilineConfig(mcfg)
	require.NoError(t, err)

	stage := &multilineStage{cfg: mcfg, regex: regex, logger: log.NewNopLogger()}

	in := make(chan Entry)
	out := stage.Run(in)

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
			in <- simpleEntry("not a start line", "label")
		}
		close(in)
	}()

	<-done
}

var mlBenchTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// BenchmarkMultilineStage benchmarks the multiline stage with a realistic
// stack-trace pattern: 1 firstline ("Date:") followed by 9 continuation lines,
// cycling continuously. Each input line amortises one-tenth of the flush cost.
func BenchmarkMultilineStage(b *testing.B) {
	origDebug := Debug
	b.Cleanup(func() { Debug = origDebug })

	for _, debugEnabled := range []bool{false, true} {
		name := "debug=false"
		if debugEnabled {
			name = "debug=true"
		}
		b.Run(name, func(b *testing.B) {
			Debug = debugEnabled

			stages := []StageConfig{
				{
					MultilineConfig: &MultilineConfig{
						Expression:   "^Date:",
						MaxWaitTime:  3 * time.Second,
						MaxLines:     128,
						TrimNewlines: true,
					},
				},
			}

			pl, err := NewPipeline(
				log.NewNopLogger(),
				stages,
				prometheus.NewRegistry(),
				featuregate.StabilityGenerallyAvailable,
			)
			if err != nil {
				b.Fatalf("NewPipeline: %v", err)
			}

			in := make(chan Entry)
			out := pl.Run(in)

			done := make(chan struct{})
			go func() {
				defer close(done)
				for range out {
				}
			}()

			const linesPerBlock = 10
			block := make([]Entry, linesPerBlock)
			block[0] = newEntry(nil, model.LabelSet{"job": "bench"},
				"Date: Mon, 01 Jan 2024 00:00:00 +0000 error occurred", mlBenchTime)
			for i := 1; i < linesPerBlock; i++ {
				block[i] = newEntry(nil, model.LabelSet{"job": "bench"},
					"\tat com.example.Foo.bar(Foo.java:42)", mlBenchTime)
			}

			b.ReportAllocs()
			b.ResetTimer()

			i := 0
			for b.Loop() {
				e := block[i%linesPerBlock]
				e.Entry.Labels = e.Entry.Labels.Clone()
				e.Extracted = make(map[string]any)
				in <- e
				i++
			}

			close(in)
			<-done
		})
	}
}

func simpleEntry(line, label string) Entry {
	// We're adding a small wait time here, because on Windows, timers have a
	// smaller resolution than on Linux. This can mess with the ordering of log
	// lines, making the test Flaky on Windows runners.
	time.Sleep(1 * time.Millisecond)
	return Entry{
		Extracted: map[string]any{},
		Entry: loki.Entry{
			Labels: model.LabelSet{"value": model.LabelValue(label)},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      line,
			},
		},
	}
}
