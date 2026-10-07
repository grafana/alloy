package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax"
)

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

// TestPipelineConsumerConcurrent runs every migrated stage in one pipeline from several
// goroutines at once to check for data races under -race.
func TestPipelineConsumerConcurrent(t *testing.T) {
	const (
		consumers          = 4
		batchesPerConsumer = 10
		streamsPerBatch    = 4
	)

	// One line shape per parser.
	const (
		ansiLine       = "\x1b[31mred\x1b[0m plain text"
		arrayLine      = `[{"a":1},{"b":2}]`
		criPartial     = "2026-09-18T10:00:00.000000001Z stderr P a partial fragment "
		criFull        = "2026-09-18T10:00:00.000000001Z stderr F user=bob token=4539148803436467 dur=1.5"
		dockerLine     = `{"log":"user=bob dur=1.5\n","stream":"stderr","time":"2026-09-18T10:00:00Z"}`
		logfmtLine     = "level=info user=bob token=4539148803436467 dur=1.5"
		jsonLine       = `{"level":"info","ip":"1.2.3.4","ts":"2026-09-18T10:00:00Z","evt":"yay","msg":"user=bob token=4539148803436467 dur=1.5"}`
		multilineStart = "START user=bob token=4539148803436467 dur=1.5"
		multilineCont  = "\tat com.example.Foo.bar(Foo.java:42)"
	)

	lines := []string{criPartial, criFull, dockerLine, jsonLine, arrayLine, logfmtLine, ansiLine, multilineStart, multilineCont}

	cfg := loadConfig(`
		stage.cri {}

		stage.docker {}

		stage.multiline {
			firstline     = "^START"
			max_lines     = 3
			max_wait_time = "10ms"
		}

		stage.match {
			selector = "{app=\"app-0\"}"

			stage.multiline {
				firstline     = "^START"
				max_lines     = 2
				max_wait_time = "10ms"
			}

			stage.decolorize {}
		}

		stage.match {
			selector            = "{instance=\"stream-0\"}"
			action              = "drop"
			drop_counter_reason = "race_test_match_drop"
		}

		stage.json {
			expressions = {
				level = "level",
				ip    = "ip",
				ts    = "ts",
				evt   = "evt",
				msg   = "msg",
			}
		}

		stage.logfmt {
			source  = "msg"
			mapping = { "user" = "", "token" = "", "dur" = "" }
		}

		stage.regex {
			source     = "msg"
			expression = "dur=(?P<dur_seconds>[0-9.]+)"
		}

		stage.pattern {
			source  = "msg"
			pattern = "user=<pattern_user> token=<pattern_token> <pattern_rest>"
		}

		stage.luhn {
			source = "msg"
		}

		stage.replace {
			source     = "msg"
			expression = "(bob)"
			replace    = "redacted-user"
		}

		stage.truncate {
			rule {
				limit       = "1000B"
				suffix      = "..."
				sources     = ["msg"]
				source_type = "extracted"
			}
		}

		stage.geoip {
			db      = "testdata/geoip_maxmind_city.mmdb"
			source  = "ip"
			db_type = "city"
		}

		stage.template {
			source   = "level"
			template = "{{ .Value }}-templated"
		}

		stage.timestamp {
			source                        = "ts"
			format                        = "RFC3339"
			action_on_duplicate_timestamp = "fudge"
		}

		stage.windowsevent {
			source              = "evt"
			drop_invalid_labels = true
			overwrite_existing  = true
		}

		stage.eventlogmessage {
			source              = "evt"
			drop_invalid_labels = true
			overwrite_existing  = true
		}

		stage.split_json {}

		stage.labels {
			values = { "level" = "" }
		}

		stage.static_labels {
			values = { "env" = "race-test", "temporary" = "dropped-below" }
		}

		stage.label_drop {
			values = ["temporary"]
		}

		stage.label_keep {
			values = ["app", "instance", "level", "env"]
		}

		stage.structured_metadata {
			values = { "user" = "" }
		}

		stage.structured_metadata_drop {
			values = ["user"]
		}

		stage.tenant {
			source = "level"
		}

		stage.limit {
			rate                = 1000000
			burst               = 1000000
			drop                = true
			by_label_name       = "app"
			max_distinct_labels = 10000
		}

		stage.sampling {
			rate = 1.0
		}

		stage.drop {
			source = "never_extracted"
			value  = "never_matches"
		}

		stage.metrics {
			metric.counter {
				name   = "race_test_lines"
				action = "inc"
				source = "level"
			}
			metric.gauge {
				name   = "race_test_duration"
				action = "set"
				source = "dur"
			}
			metric.histogram {
				name    = "race_test_duration_hist"
				source  = "dur"
				buckets = [0.5, 1, 2]
			}
		}

		stage.output {
			source = "msg"
		}

		stage.decolorize {}

		stage.pack {
			labels           = ["env"]
			ingest_timestamp = true
		}
	`)

	pc, err := NewPipelineConsumer(
		slog.New(slog.DiscardHandler),
		prometheus.NewRegistry(),
		featuregate.StabilityGenerallyAvailable,
		cfg,
		loki.NewNopConsumer(),
	)
	require.NoError(t, err)

	var (
		wg   sync.WaitGroup
		errs = make([]error, consumers)
	)

	for i := range consumers {
		wg.Go(func() {
			for range batchesPerConsumer {
				batch := loki.NewBatch()
				for s := range streamsPerBatch {
					// app is capped to half the consumer count so goroutines contend for the
					// same rate limiter in limit and the same fingerprint in cri and timestamp.
					labels := model.LabelSet{
						"app":      model.LabelValue(fmt.Sprintf("app-%d", (i+s)%consumers/2)),
						"instance": model.LabelValue(fmt.Sprintf("stream-%d", s)),
					}

					for _, line := range lines {
						batch.AddEntry(labels, time.Now().UnixMicro(), push.Entry{
							Timestamp: time.Now(),
							Line:      line,
						})
					}
				}

				if err := pc.Consume(context.Background(), batch); err != nil {
					errs[i] = err
					return
				}
			}
		})
	}

	wg.Wait()
	pc.Stop()
	require.NoError(t, errors.Join(errs...))
}

func TestNewPipelineStopsOnFailure(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	cfgs := loadConfig(`
	stage.regex {
		expression = "[unclosed"
	}
	stage.match {
		selector = "{app=\"x\"}"
		action   = "keep"

		stage.multiline {
			firstline     = "^START"
			max_wait_time = "10ms"
		}
	}
	stage.multiline {
		firstline     = "^START"
		max_wait_time = "10ms"
	}
	`)

	next := func(_ context.Context, _ []Entry) error { return nil }
	_, err := newPipeline(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
	require.Error(t, err)
}

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
