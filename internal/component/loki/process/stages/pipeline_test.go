package stages

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax"
)

// Configs defines multiple StageConfigs as consequent blocks.
type Configs struct {
	Stages []StageConfig `alloy:"stage,enum,optional"`
}

func loadConfig(cfg string) []StageConfig {
	var config Configs
	err := syntax.Unmarshal([]byte(cfg), &config)
	if err != nil {
		panic(err)
	}
	return config.Stages
}

type entryCheckFNs struct {
	metrics             func(reg *prometheus.Registry) error
	metricsAfterCleanup func(reg *prometheus.Registry) error
	timestamp           func(expected, actual time.Time) bool
	extracted           func(expected, actual map[string]any) bool
	structuredMetadata  func(expected, actual push.LabelsAdapter) bool
}

// runPipelineTest builds a pipeline for cfgs, runs entries through it, and
// asserts the result matches expected. checks is optional and lets a caller
// override how individual fields are compared.
// metrics checks the registry once entries have been processed, and
// metricsAfterCleanup checks it again after the pipeline has been stopped.
// Both are skipped when nil.
func runPipelineTest(t *testing.T, cfgs []StageConfig, entries []Entry, expected []Entry, checks ...entryCheckFNs) {
	var check entryCheckFNs
	if len(checks) > 0 {
		check = checks[0]
	}

	registry := prometheus.NewRegistry()

	var (
		collected    []Entry
		collectedMut sync.Mutex
	)

	next := func(_ context.Context, entries []Entry) error {
		collectedMut.Lock()
		defer collectedMut.Unlock()
		collected = append(collected, entries...)
		return nil
	}

	p, err := newPipeline(logging.NewSlogNop(), registry, featuregate.StabilityGenerallyAvailable, cfgs, next)
	require.NoError(t, err)
	require.NoError(t, p.process(context.Background(), entries))

	if check.metrics != nil {
		require.NoError(t, check.metrics(registry))
	}

	p.stop()
	collectedMut.Lock()
	defer collectedMut.Unlock()
	assertEntriesUnordered(t, expected, collected, check)

	if check.metricsAfterCleanup != nil {
		require.NoError(t, check.metricsAfterCleanup(registry))
	}
}

// TODO(kalleep): PipelineConsumer needs the same semantics, Stop must release
// an entry blocked inside a stage. Cover it here once it is implemented.
func TestPipelineStopReleasesBlockedStage(t *testing.T) {
	type testCase struct {
		name string
		cfg  string
	}

	tests := []testCase{
		{
			name: "stage.limit",
			cfg: `
			stage.limit {
				rate  = 0.1
				burst = 1
				drop  = false
			}
			`,
		},
		{
			name: "stage.limit inside stage.match",
			cfg: `
			stage.match {
				selector = "{app=\"loki\"}"
				action = "keep"
				stage.limit {
					rate  = 0.1
					burst = 1
					drop  = false
				}
			}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl, err := NewPipeline(logging.NewSlogNop(), loadConfig(tt.cfg), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)

			in := make(chan loki.Entry)
			out := make(chan loki.Entry, 1)
			pl.Start(in, out)

			entry := loki.Entry{
				Labels: model.LabelSet{"app": "loki"},
				Entry:  push.Entry{Line: testMatchLogLineApp1, Timestamp: time.Now()},
			}

			in <- entry
			<-out       // burst consumed; next Wait() will block
			in <- entry // blocks the limit stage in rateLimiter.Wait

			done := make(chan struct{})
			go func() {
				defer close(done)
				pl.Stop()
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Stop() did not release the entry blocked in rateLimiter.Wait")
			}

			select {
			case e := <-out:
				t.Fatalf("expected the entry blocked in rateLimiter.Wait to be dropped on shutdown, but it was forwarded: %+v", e)
			default:
			}
		})
	}
}

func runPipelineBenchmark(b *testing.B, cfgs []StageConfig, batches []loki.Batch) {
	distribute := func(n int, batches []loki.Batch, work func(worker, iters int)) {
		var (
			wg         sync.WaitGroup
			numWorkers = len(batches)
		)
		itersPerWorker, remainder := n/numWorkers, n%numWorkers
		for w := 0; w < numWorkers; w++ {
			iters := itersPerWorker
			if w < remainder {
				iters++
			}
			wg.Go(func() {
				work(w, iters)
			})
		}
		wg.Wait()
	}

	b.Run("Pipeline", func(b *testing.B) {
		p, err := NewPipeline(logging.NewSlogNop(), cfgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(b, err)

		in := make(chan loki.Entry)
		out := make(chan loki.Entry)
		p.Start(in, out)
		defer close(out)
		defer p.Stop()

		go func() {
			for range out {
			}
		}()

		workerEntries := make([][]loki.Entry, len(batches))
		for w, batch := range batches {
			clone := batch.Clone()
			entries := make([]loki.Entry, 0, clone.EntryLen())
			for _, stream := range clone.Streams() {
				for _, e := range stream.Entries {
					entries = append(entries, loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), stream.Created(), e))
				}
			}
			workerEntries[w] = entries
		}

		b.ResetTimer()
		b.ReportAllocs()

		distribute(b.N, batches, func(worker, iters int) {
			entries := workerEntries[worker]
			for i := 0; i < iters; i++ {
				for _, e := range entries {
					in <- e.Clone()
				}
			}
		})
	})

	b.Run("PipelineConsumer", func(b *testing.B) {
		consumer := loki.NewNopConsumer()
		pc, err := NewPipelineConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, consumer)
		require.NoError(b, err)
		defer pc.Stop()

		b.ResetTimer()
		b.ReportAllocs()

		distribute(b.N, batches, func(worker, iters int) {
			batch := batches[worker]
			for i := 0; i < iters; i++ {
				_ = pc.Consume(context.Background(), batch.Clone())
			}
		})
	})
}

// assertEntriesUnordered asserts that actual contains exactly the entries in
// expected, ignoring order.
func assertEntriesUnordered(t require.TestingT, expected, actual []Entry, checks entryCheckFNs) {
	require.Len(t, actual, len(expected))

	entriesEqual := func(expected, actual Entry) bool {
		if expected.Line != actual.Line {
			return false
		}

		if checks.timestamp != nil {
			if !checks.timestamp(expected.Timestamp, actual.Timestamp) {
				return false
			}
		} else {
			if expected.Timestamp.UnixNano() != actual.Timestamp.UnixNano() {
				return false
			}
		}

		if !reflect.DeepEqual(expected.Labels, actual.Labels) {
			return false
		}

		if checks.extracted != nil {
			if !checks.extracted(expected.Extracted, actual.Extracted) {
				return false
			}
		} else {
			if !reflect.DeepEqual(expected.Extracted, actual.Extracted) {
				return false
			}
		}

		var (
			expectedStructured = slices.Clone(expected.StructuredMetadata)
			actualStructured   = slices.Clone(actual.StructuredMetadata)
		)

		sortLabelAdapters := func(s []push.LabelAdapter) {
			slices.SortFunc(s, func(a, b push.LabelAdapter) int {
				if a.Name != b.Name {
					return strings.Compare(a.Name, b.Name)
				}
				return strings.Compare(a.Value, b.Value)
			})
		}
		sortLabelAdapters(expectedStructured)
		sortLabelAdapters(actualStructured)

		if checks.structuredMetadata != nil {
			return checks.structuredMetadata(expectedStructured, actualStructured)
		}
		return reflect.DeepEqual(expectedStructured, actualStructured)
	}

	remaining := append([]Entry(nil), actual...)
	for _, exp := range expected {
		found := -1
		for i, got := range remaining {
			if entriesEqual(exp, got) {
				found = i
				break
			}
		}

		require.NotEqual(t, -1, found, "no matching entry found for expected entry: %+v", exp)
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
}
