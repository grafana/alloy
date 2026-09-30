package stages

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
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

// Configs defines multiple StageConfigs as consequent blocks.
type Configs struct {
	Stages []StageConfig `alloy:"stage,enum,optional"`
}

func withInboundEntries(entries ...Entry) chan Entry {
	in := make(chan Entry, len(entries))
	defer close(in)
	for _, e := range entries {
		in <- e
	}
	return in
}

func processEntries(s Stage, entries ...Entry) []Entry {
	out := s.Run(withInboundEntries(entries...))
	var res []Entry
	for e := range out {
		res = append(res, e)
	}
	return res
}

func loadConfig(cfg string) []StageConfig {
	var config Configs
	err := syntax.Unmarshal([]byte(cfg), &config)
	if err != nil {
		panic(err)
	}
	return config.Stages
}

func newPipelineFromConfig(cfg string) (*Pipeline, error) {
	return NewPipeline(logging.NewSlogNop(), loadConfig(cfg), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
}

type entryCheckFNs struct {
	metrics             func(reg *prometheus.Registry) error
	metricsAfterCleanup func(reg *prometheus.Registry) error
	timestamp           func(expected, actual time.Time) bool
	extracted           func(expected, actual map[string]any) bool
	structuredMetadata  func(expected, actual push.LabelsAdapter) bool
}

// runPipelineTest builds a pipeline for cfgs using both the old and new
// pipeline implementations, runs entries through each, and asserts the
// result matches expected. checks is optional and lets a caller override how
// individual fields are compared.
// metrics checks the registry once entries have been processed, and
// metricsAfterCleanup checks it again after the pipeline's Cleanup() has run.
// Both are skipped when nil.
func runPipelineTest(t *testing.T, cfgs []StageConfig, entries []Entry, expected []Entry, checks ...entryCheckFNs) {
	var check entryCheckFNs
	if len(checks) > 0 {
		check = checks[0]
	}

	cloned := cloneEntries(entries)

	t.Run("Pipeline", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		p, err := NewPipeline(logging.NewSlogNop(), cfgs, registry, featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)

		var collected []Entry
		for e := range p.Run(withInboundEntries(cloned...)) {
			collected = append(collected, e)
		}

		if check.metrics != nil {
			require.NoError(t, check.metrics(registry))
		}

		p.Stop()
		p.Cleanup()

		assertEntriesUnordered(t, expected, collected, check)

		if check.metricsAfterCleanup != nil {
			require.NoError(t, check.metricsAfterCleanup(registry))
		}
	})

	t.Run("New Pipeline", func(t *testing.T) {
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
	})
}

func cloneEntries(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	for i, e := range entries {
		out[i] = Entry{
			Extracted: maps.Clone(e.Extracted),
			Entry:     e.Entry.Clone(),
		}
	}
	return out
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
		handler := p.Start(in, out)

		defer close(out)
		defer handler.Stop()

		go func() {
			for range out {
			}
		}()

		workerEntries := make([][]loki.Entry, len(batches))
		for w, batch := range batches {
			clone := batch.Clone()
			entries := make([]loki.Entry, 0, clone.EntryLen())
			_ = clone.ConsumeStreams(func(stream loki.Stream) error {
				for _, e := range stream.Entries {
					entries = append(entries, loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), stream.Created(), e))
				}
				return nil
			})
			workerEntries[w] = entries
		}

		b.ResetTimer()
		b.ReportAllocs()

		distribute(b.N, batches, func(worker, iters int) {
			entries := workerEntries[worker]
			for i := 0; i < iters; i++ {
				for _, e := range entries {
					handler.Chan() <- e.Clone()
				}
			}
		})
	})

	b.Run("New Pipeline", func(b *testing.B) {
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

var (
	rawTestLine       = `{"log":"11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] \"GET /1986.js HTTP/1.1\" 200 932 \"-\" \"Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6\"","stream":"stderr","time":"2019-04-30T02:12:41.8443515Z"}`
	processedTestLine = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`
)

var testMultiStageAlloy = `
stage.match {
		selector = "{match=\"true\"}"
		stage.docker {}
		stage.regex {
				expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
		}
		stage.regex {
				source     = "filename"
				expression = "(?P<service>[^\\/]+)\\.log"
		}
		stage.timestamp {
				source = "timestamp"
				format = "02/Jan/2006:15:04:05 -0700"
		}
		stage.labels {
				values = { "action" = "", "service" = "", "status_code" = "status" }
		}
}
stage.match {
		selector = "{match=\"false\"}"
		action   = "drop"
}`

var testLabelsFromJSONAlloy = `
stage.json {
		expressions = { "app" = "", "message" = "" }
}
stage.labels {
		values = { "app" = "" }
}
stage.output {
		source = "message"
}`

func TestPipeline(t *testing.T) {
	t.Parallel()

	var (
		now = time.Now()
	)

	est, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	processedTS := time.Date(2000, 01, 25, 14, 00, 01, 0, est)

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name:   "happy path",
			config: testMultiStageAlloy,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"match": "true",
				}, rawTestLine, time.Now()),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"match":     "true",
					"output":    processedTestLine,
					"stream":    "stderr",
					"timestamp": "25/Jan/2000:14:00:01 -0500",
					"ip":        "11.11.11.11",
					"identd":    "-",
					"user":      "frank",
					"action":    "GET",
					"path":      "/1986.js",
					"protocol":  "HTTP/1.1",
					"status":    "200",
					"size":      "932",
					"referer":   "-",
					"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
				}, model.LabelSet{
					"match":       "true",
					"stream":      "stderr",
					"action":      "GET",
					"status_code": "200",
				}, processedTestLine, processedTS),
			},
		},
		{
			name:   "no match",
			config: testMultiStageAlloy,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"nomatch": "true",
				}, rawTestLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"nomatch": "true",
				}, model.LabelSet{
					"nomatch": "true",
				}, rawTestLine, now),
			},
		},
		{
			name:   "should initialize the extracted map with the initial labels",
			config: testMultiStageAlloy,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"match":    "true",
					"filename": "/var/log/nginx/frontend.log",
				}, rawTestLine, time.Now()),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"match":     "true",
					"filename":  "/var/log/nginx/frontend.log",
					"service":   "frontend",
					"output":    processedTestLine,
					"stream":    "stderr",
					"timestamp": "25/Jan/2000:14:00:01 -0500",
					"ip":        "11.11.11.11",
					"identd":    "-",
					"user":      "frank",
					"action":    "GET",
					"path":      "/1986.js",
					"protocol":  "HTTP/1.1",
					"status":    "200",
					"size":      "932",
					"referer":   "-",
					"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
				}, model.LabelSet{
					"filename":    "/var/log/nginx/frontend.log",
					"match":       "true",
					"stream":      "stderr",
					"service":     "frontend",
					"action":      "GET",
					"status_code": "200",
				}, processedTestLine, processedTS),
			},
		},
		{
			name:   "should set a label from value extracted from JSON",
			config: testLabelsFromJSONAlloy,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"message":"hello world","app":"api"}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"app":     "api",
					"message": "hello world",
				}, model.LabelSet{
					"app": "api",
				}, "hello world", now),
			},
		},
		{
			name:   "should not set a label if the field does not exist in the JSON",
			config: testLabelsFromJSONAlloy,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"message":"hello world"}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"app":     nil,
					"message": "hello world",
				}, model.LabelSet{}, "hello world", now),
			},
		},
		{
			name:   "should not set a label if the value extracted from JSON is null",
			config: testLabelsFromJSONAlloy,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"message":"hello world","app":null}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"app":     nil,
					"message": "hello world",
				}, model.LabelSet{}, "hello world", now),
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

func TestPipeline_Start(t *testing.T) {
	now := time.Now()
	p, err := NewPipeline(logging.NewSlogNop(), loadConfig(testMultiStageAlloy), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)

	type testCase struct {
		name       string
		labels     model.LabelSet
		shouldSend bool
	}

	tests := []testCase{
		{
			name: "should drop",
			labels: model.LabelSet{
				"stream":      "stderr",
				"action":      "GET",
				"status_code": "200",
				"match":       "false",
			},
			shouldSend: false,
		},
		{
			name: "should send",
			labels: model.LabelSet{
				"stream":      "stderr",
				"action":      "GET",
				"status_code": "200",
			},
			shouldSend: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := loki.NewCollectingHandler()
			handler := p.Start(make(chan loki.Entry), c.Chan())

			handler.Chan() <- loki.Entry{
				Labels: tt.labels,
				Entry: push.Entry{
					Line:      rawTestLine,
					Timestamp: now,
				},
			}
			handler.Stop()
			c.Stop()
			var received bool

			if len(c.Received()) != 0 {
				received = true
			}

			assert.Equal(t, tt.shouldSend, received)
		})
	}
}

func TestPipelineConcurrent(t *testing.T) {
	cfg := `
stage.match {
		selector = "{match=~\".*\"}"
		stage.multiline {
				firstline     = "^{"
				max_wait_time = "3s"
				max_lines     = 2
		}
		stage.json {
				expressions = { "app" = "", "message" = "" }
		}
		stage.labels {
				values = { "app" = "" }
		}
		stage.output {
				source = "message"
		}
}
stage.match {
		selector = "{match=~\".*\"}"
		stage.json {
				expressions = { "app" = "", "message" = "" }
		}
		stage.labels {
				values = { "app" = "" }
			}
		stage.output {
				source = "message"
			}
}
`
	p, err := newPipelineFromConfig(cfg)
	require.NoError(t, err)

	out := loki.NewCollectingHandler()

	e1 := p.Start(make(chan loki.Entry), out.Chan())
	e2 := loki.AddLabelsMiddleware(model.LabelSet{"bar": "foo"}).Wrap(e1)
	entryhandler := loki.AddLabelsMiddleware(model.LabelSet{"foo": "bar"}).Wrap(e2)

	const parallelism = 10

	sent := make([][]string, parallelism)
	for i := range parallelism {
		sent[i] = []string{
			fmt.Sprintf(`{app:"%d", `, i),
			fmt.Sprintf(` message:"%d"}`, i),
		}
	}

	var wg sync.WaitGroup
	wg.Add(parallelism)

	for i := range parallelism {
		go func(i int) {
			defer wg.Done()
			for _, line := range sent[i] {
				entryhandler.Chan() <- loki.Entry{
					Labels: make(model.LabelSet),
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      line,
					},
				}
			}
		}(i)
	}

	wg.Wait()
	entryhandler.Stop()
	e2.Stop()
	e1.Stop()
	out.Stop()

	// The middlewares give every producer the same labels, so all lines land in
	// one multiline stream and which lines end up in a block depends on the
	// interleaving. What must hold is that no line is dropped or duplicated.
	var got []string
	for _, e := range out.Received() {
		got = append(got, strings.Split(e.Line, "\n")...)
	}

	var expected []string
	for _, lines := range sent {
		expected = append(expected, lines...)
	}

	slices.Sort(got)
	slices.Sort(expected)
	require.Equal(t, expected, got)
}

var (
	infoLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}))
	debugLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}))
)

func BenchmarkPipeline(b *testing.B) {
	type testCase struct {
		name   string
		cfg    string
		line   string
		logger *slog.Logger
	}

	tests := []testCase{
		{
			name:   "two stage info level",
			cfg:    testMultiStageAlloy,
			line:   rawTestLine,
			logger: infoLogger,
		},
		{
			name:   "two stage debug level",
			cfg:    testMultiStageAlloy,
			line:   rawTestLine,
			logger: debugLogger,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			pl, err := NewPipeline(tt.logger, loadConfig(tt.cfg), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			require.NoError(b, err)

			lb := model.LabelSet{}
			ts := time.Now()

			in := make(chan loki.Entry)
			out := make(chan loki.Entry)
			handler := pl.Start(in, out)
			defer handler.Stop()
			b.ResetTimer()

			go func() {
				for range out {
				}
			}()

			for b.Loop() {
				in <- loki.NewEntry(lb.Clone(), push.Entry{Timestamp: ts, Line: tt.line})
			}

			close(in)
		})
	}
}
