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
	"github.com/prometheus/client_golang/prometheus/testutil"
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

func loadConfig(yml string) []StageConfig {
	var config Configs
	err := syntax.Unmarshal([]byte(yml), &config)
	if err != nil {
		panic(err)
	}
	return config.Stages
}

func newPipelineFromConfig(cfg string) (*Pipeline, error) {
	return NewPipeline(logging.NewSlogNop(), loadConfig(cfg), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
}

type entryCheckFNs struct {
	timestamp          func(expected, actual time.Time) bool
	extracted          func(expected, actual map[string]any) bool
	structuredMetadata func(expected, actual push.LabelsAdapter) bool
}

// runPipelineTest builds a pipeline for cfgs using both the old and new
// pipeline implementations, runs entries through each, and asserts the
// result matches expected. expectedMetrics is optional: pass "" to skip
// checking metrics, or a Prometheus exposition-format string (as consumed
// by testutil.GatherAndCompare) to assert against each run's own registry.
func runPipelineTest(t *testing.T, cfgs []StageConfig, entries []Entry, expected []Entry, expectedMetrics string, checks ...entryCheckFNs) {
	var check entryCheckFNs
	if len(checks) > 0 {
		check = checks[0]
	}

	// Pipeline.Run seeds the extracted map with each entry's initial labels
	// before running any stage. process (called directly below, bypassing
	// ProcessBatch/ProcessEntry) does not. Seed it here once so both
	// pipeline implementations start from the same state.
	for i := range entries {
		for labelName, labelValue := range entries[i].Labels {
			entries[i].Extracted[string(labelName)] = string(labelValue)
		}
	}

	cloneEntries := func(entries []Entry) []Entry {
		out := make([]Entry, len(entries))
		for i, e := range entries {
			out[i] = Entry{
				Extracted: maps.Clone(e.Extracted),
				Entry:     e.Entry.Clone(),
			}
		}
		return out
	}

	cloned := cloneEntries(entries)

	t.Run("Stage", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		p, err := NewPipeline(logging.NewSlogNop(), cfgs, registry, featuregate.StabilityGenerallyAvailable)
		require.NoError(t, err)
		defer p.Cleanup()
		defer p.Stop()

		out := p.Run(withInboundEntries(cloned...))
		var collected []Entry
		for e := range out {
			collected = append(collected, e)
		}

		assertEntriesUnordered(t, expected, collected, check)
		if expectedMetrics != "" {
			require.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expectedMetrics)))
		}
	})

	t.Run("New Stage", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		var collected []Entry
		next := func(_ context.Context, entries []Entry) error {
			collected = append(collected, entries...)
			return nil
		}

		p, err := NewPipeline2(logging.NewSlogNop(), registry, featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(t, err)
		defer p.Stop()

		p.process(context.Background(), entries)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			assertEntriesUnordered(c, expected, collected, check)
			if expectedMetrics != "" {
				assert.NoError(c, testutil.GatherAndCompare(registry, strings.NewReader(expectedMetrics)))
			}
		}, 2*time.Second, 100*time.Millisecond)
	})
}

// benchResultLokiEntry and benchResultEntries sink runPipelineBenchmark's
// results so the compiler can't optimize the calls being measured away.
var (
	benchResultEntries   []Entry
	benchResultLokiEntry loki.Entry
)

// runPipelineBenchmark benchmarks a pipeline built for cfgs against batch.
// It will run one benchmark for new stage implementation and one for old implementation.
func runPipelineBenchmark(b *testing.B, cfgs []StageConfig, batch loki.Batch) {
	b.Run("Stage", func(b *testing.B) {
		p, err := NewPipeline(logging.NewSlogNop(), cfgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		require.NoError(b, err)

		in := make(chan loki.Entry)
		out := make(chan loki.Entry)
		handler := p.Start(in, out)

		done := make(chan struct{})
		go func() {
			defer close(done)
			for e := range out {
				benchResultLokiEntry = e
			}
		}()

		clone := batch.Clone()
		entries := make([]loki.Entry, 0, clone.EntryLen())
		_ = clone.ConsumeStreams(func(stream loki.Stream, created int64) error {
			for _, e := range stream.Entries {
				entries = append(entries, loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), created, e))
			}
			return nil
		})

		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			for _, e := range entries {
				handler.Chan() <- e.Clone()
			}
		}
		b.StopTimer()

		handler.Stop()
		close(out)
		<-done
	})

	b.Run("New Stage", func(b *testing.B) {
		next := func(_ context.Context, e []Entry) error {
			benchResultEntries = e
			return nil
		}

		p, err := NewPipeline2(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, cfgs, next)
		require.NoError(b, err)
		defer p.Stop()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = p.ProcessBatch(context.Background(), batch.Clone())
		}
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

// TODO(@tpaschalis) Comment these out until we port over the remaining
// stages and use these tests to verify their behavior.
var (
	ct                = time.Now()
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

func TestNewPipeline(t *testing.T) {
	p, err := NewPipeline(logging.NewSlogNop(), loadConfig(testMultiStageAlloy), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		panic(err)
	}
	require.Len(t, p.stages, 2)
}

func TestPipeline_Process(t *testing.T) {
	t.Parallel()

	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal("could not parse timestamp", err)
	}

	tests := map[string]struct {
		config         string
		entry          string
		expectedEntry  string
		t              time.Time
		expectedT      time.Time
		initialLabels  model.LabelSet
		expectedLabels model.LabelSet
	}{
		"happy path": {
			testMultiStageAlloy,
			rawTestLine,
			processedTestLine,
			time.Now(),
			time.Date(2000, 01, 25, 14, 00, 01, 0, est),
			map[model.LabelName]model.LabelValue{
				"match": "true",
			},
			map[model.LabelName]model.LabelValue{
				"match":       "true",
				"stream":      "stderr",
				"action":      "GET",
				"status_code": "200",
			},
		},
		"no match": {
			testMultiStageAlloy,
			rawTestLine,
			rawTestLine,
			ct,
			ct,
			map[model.LabelName]model.LabelValue{
				"nomatch": "true",
			},
			map[model.LabelName]model.LabelValue{
				"nomatch": "true",
			},
		},
		"should initialize the extracted map with the initial labels": {
			testMultiStageAlloy,
			rawTestLine,
			processedTestLine,
			time.Now(),
			time.Date(2000, 01, 25, 14, 00, 01, 0, est),
			map[model.LabelName]model.LabelValue{
				"match":    "true",
				"filename": "/var/log/nginx/frontend.log",
			},
			map[model.LabelName]model.LabelValue{
				"filename":    "/var/log/nginx/frontend.log",
				"match":       "true",
				"stream":      "stderr",
				"service":     "frontend",
				"action":      "GET",
				"status_code": "200",
			},
		},
		"should set a label from value extracted from JSON": {
			testLabelsFromJSONAlloy,
			`{"message":"hello world","app":"api"}`,
			"hello world",
			ct,
			ct,
			map[model.LabelName]model.LabelValue{},
			map[model.LabelName]model.LabelValue{
				"app": "api",
			},
		},
		"should not set a label if the field does not exist in the JSON": {
			testLabelsFromJSONAlloy,
			`{"message":"hello world"}`,
			"hello world",
			ct,
			ct,
			map[model.LabelName]model.LabelValue{},
			map[model.LabelName]model.LabelValue{},
		},
		"should not set a label if the value extracted from JSON is null": {
			testLabelsFromJSONAlloy,
			`{"message":"hello world","app":null}`,
			"hello world",
			ct,
			ct,
			map[model.LabelName]model.LabelValue{},
			map[model.LabelName]model.LabelValue{},
		},
	}

	for tName, tt := range tests {
		tt := tt

		t.Run(tName, func(t *testing.T) {
			var config Configs

			err := syntax.Unmarshal([]byte(tt.config), &config)
			require.NoError(t, err)

			p, err := NewPipeline(logging.NewSlogNop(), loadConfig(tt.config), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)

			out := processEntries(p, newEntry(nil, tt.initialLabels, tt.entry, tt.t))[0]

			assert.Equal(t, tt.expectedLabels, out.Labels, "did not get expected labels")
			assert.Equal(t, tt.expectedEntry, out.Line, "did not receive expected log entry")
			if out.Timestamp.Unix() != tt.expectedT.Unix() {
				t.Fatalf("mismatch ts want: %s got:%s", tt.expectedT, tt.t)
			}
		})
	}
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
	benchmarks := []struct {
		name   string
		stgs   []StageConfig
		logger *slog.Logger
		entry  string
	}{
		{
			"two stage info level",
			loadConfig(testMultiStageAlloy),
			infoLogger,
			rawTestLine,
		},
		{
			"two stage debug level",
			loadConfig(testMultiStageAlloy),
			debugLogger,
			rawTestLine,
		},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			pl, err := NewPipeline(logging.NewSlogNop(), bm.stgs, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				panic(err)
			}
			lb := model.LabelSet{}
			ts := time.Now()

			in := make(chan Entry)
			out := pl.Run(in)
			b.ResetTimer()

			go func() {
				for range out {
				}
			}()
			for b.Loop() {
				in <- newEntry(nil, lb, bm.entry, ts)
			}
			close(in)
		})
	}
}

func TestPipeline_Wrap(t *testing.T) {
	now := time.Now()
	p, err := NewPipeline(logging.NewSlogNop(), loadConfig(testMultiStageAlloy), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		panic(err)
	}

	tests := map[string]struct {
		labels     model.LabelSet
		shouldSend bool
	}{
		"should drop": {
			map[model.LabelName]model.LabelValue{
				"stream":      "stderr",
				"action":      "GET",
				"status_code": "200",
				"match":       "false",
			},
			false,
		},
		"should send": {
			map[model.LabelName]model.LabelValue{
				"stream":      "stderr",
				"action":      "GET",
				"status_code": "200",
			},
			true,
		},
	}

	for tName, tt := range tests {
		t.Run(tName, func(t *testing.T) {
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

func Test_PipelineParallel(t *testing.T) {
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

	var wg sync.WaitGroup
	parallelism := 10
	wg.Add(parallelism)

	for i := range parallelism {
		go func(i int) {
			defer wg.Done()
			entryhandler.Chan() <- loki.Entry{
				Labels: make(model.LabelSet),
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      fmt.Sprintf(`{app:"%d", `, 5),
				},
			}
			entryhandler.Chan() <- loki.Entry{
				Labels: make(model.LabelSet),
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      fmt.Sprintf(` message:"%s"}`, time.Now()),
				},
			}
			t.Log(i)
		}(i)
	}

	wg.Wait()
	entryhandler.Stop()
	e2.Stop()
	e1.Stop()
	out.Stop()
	t.Log(out.Received())
}
