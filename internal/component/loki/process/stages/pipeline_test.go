package stages

import (
	"fmt"
	"io"
	"log/slog"
	"maps"
	"runtime"
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

// genRulesConfig builds numRules independent stage.match blocks, each gated
// on a distinct label value and containing one small nested stage. This
// mirrors a pipeline assembled from many separately configured rules (for
// example, one rule per tenant policy) rather than a handful of stages
// written by hand, so it can be scaled up to see how per-entry cost scales
// with rule count.
func genRulesConfig(numRules int) string {
	var sb strings.Builder
	for i := 0; i < numRules; i++ {
		fmt.Fprintf(&sb, `
stage.match {
	selector = "{rule_id=\"rule-%d\"}"
	stage.static_labels {
		values = {
			rule_matched = "%d",
		}
	}
}
`, i, i)
	}
	return sb.String()
}

// BenchmarkPipelineManyRules measures how per-entry latency scales with the
// number of sequential stages in a pipeline. Every entry pays the full
// traversal cost of checking each rule's selector regardless of whether it
// ends up matching one — a matching and a non-matching entry were measured
// separately and found equivalent, so this only exercises the non-matching
// case.
func BenchmarkPipelineManyRules(b *testing.B) {
	ruleCounts := []int{1, 10, 50, 100, 500, 1000}

	for _, numRules := range ruleCounts {
		stgs := loadConfig(genRulesConfig(numRules))

		b.Run(fmt.Sprintf("rules=%d", numRules), func(b *testing.B) {
			lb := model.LabelSet{"rule_id": "no-match"}
			benchmarkPipelineEntries(b, stgs, lb, rawTestLine)
		})
	}
}

// genRulesConfigWithOneMultiline is genRulesConfig, but the rule at index
// multilineAt nests a stage.multiline instead of stage.static_labels.
// multiline needs its own goroutine — it must flush a stale block even when
// no new entry arrives — so exactly that one rule falls back to its own
// channel (see newMatcherStage); every other rule still fuses into a plain
// function call. See BenchmarkPipelineOneMultilineAmongManyRules.
func genRulesConfigWithOneMultiline(numRules, multilineAt int) string {
	var sb strings.Builder
	for i := 0; i < numRules; i++ {
		if i == multilineAt {
			fmt.Fprintf(&sb, `
stage.match {
	selector = "{rule_id=\"rule-%d\"}"
	stage.multiline {
		firstline     = "^NEVER_MATCHES"
		max_wait_time = "3s"
	}
}
`, i)
			continue
		}
		fmt.Fprintf(&sb, `
stage.match {
	selector = "{rule_id=\"rule-%d\"}"
	stage.static_labels {
		values = {
			rule_matched = "%d",
		}
	}
}
`, i, i)
	}
	return sb.String()
}

// BenchmarkPipelineOneMultilineAmongManyRules checks that one rule needing
// its own channel (because it nests stage.multiline) doesn't drag the rest
// of a large rule set back into the old per-stage-channel behavior: only the
// fusion immediately around that one rule is affected, not the whole
// pipeline. "all_sync" (no multiline anywhere) is the reference point.
//
// "in_middle" tends to measure faster than "all_sync" under a small
// GOMAXPROCS: splitting ~1000 fused rules into two ~500-rule halves turns
// them into two independent goroutines that can pipeline different entries
// across separate cores at once, an incidental win from an even split
// rather than something this design specifically provides — it disappears
// under GOMAXPROCS=1, where all four variants converge. "at_start" and
// "at_end" don't get this, because one side of the split is nearly the
// entire rule set and the other is nearly empty, so there's nothing to
// overlap. The number that matters here is that none of these are anywhere
// near the cost of giving every one of the 1000 rules its own channel.
func BenchmarkPipelineOneMultilineAmongManyRules(b *testing.B) {
	const n = 1000
	variants := []struct {
		name string
		cfg  string
	}{
		{"all_sync", genRulesConfig(n)},
		{"one_multiline_at_start", genRulesConfigWithOneMultiline(n, 0)},
		{"one_multiline_in_middle", genRulesConfigWithOneMultiline(n, n/2)},
		{"one_multiline_at_end", genRulesConfigWithOneMultiline(n, n-1)},
	}

	for _, v := range variants {
		b.Run(v.name, func(b *testing.B) {
			stgs := loadConfig(v.cfg)
			lb := model.LabelSet{"rule_id": "no-match"}
			benchmarkPipelineEntries(b, stgs, lb, rawTestLine)
		})
	}
}

func benchmarkPipelineEntries(b *testing.B, stgs []StageConfig, lb model.LabelSet, entry string) {
	pl, err := NewPipeline(logging.NewSlogNop(), stgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
	if err != nil {
		panic(err)
	}
	ts := time.Now()

	in := make(chan Entry)
	out := pl.Run(in)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range out {
		}
	}()

	b.ResetTimer()
	for b.Loop() {
		// The pipeline may still be processing an earlier entry when this
		// send returns, so every entry needs its own label map: a matching
		// rule mutates Labels in place, and sharing one map across entries
		// that are processed concurrently races.
		in <- newEntry(nil, maps.Clone(lb), entry, ts)
	}
	close(in)
	// b.Loop already stopped the timer, so waiting for the last entries to
	// finish draining here doesn't affect ns/op — it just makes sure this
	// sub-benchmark's goroutines are gone before the next one starts,
	// instead of bleeding CPU time into its measurement.
	<-done
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

// BenchmarkPipelineManyStreamsSaturated models many concurrent log streams
// actually competing for the CPU, as opposed to BenchmarkPipelineManyRules'
// single stream with idle cores to spare. The two regimes favor opposite
// designs: with idle cores available, spreading tiny amounts of relay work
// across them can make a single stream's latency look fine even if it's
// wasteful, since there's nothing else for those cores to do. Once enough
// streams are genuinely competing for the same cores, that per-stage
// goroutine overhead has nowhere to hide and becomes a direct tax on
// aggregate throughput, which this benchmark's reported entries/sec makes
// visible.
func BenchmarkPipelineManyStreamsSaturated(b *testing.B) {
	const (
		numRules      = 1000
		entriesPerRun = 200
	)
	streams := runtime.GOMAXPROCS(0) * 4

	stgs := loadConfig(genRulesConfig(numRules))
	pipelines := make([]*Pipeline, streams)
	for i := range pipelines {
		pl, err := NewPipeline(logging.NewSlogNop(), stgs, prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
		if err != nil {
			b.Fatal(err)
		}
		pipelines[i] = pl
	}
	lb := model.LabelSet{"rule_id": "no-match"}
	ts := time.Now()

	b.ResetTimer()
	for b.Loop() {
		var wg sync.WaitGroup
		wg.Add(streams)
		for _, pl := range pipelines {
			go func(pl *Pipeline) {
				defer wg.Done()

				in := make(chan Entry)
				out := pl.Run(in)
				done := make(chan struct{})
				go func() {
					defer close(done)
					for range out {
					}
				}()

				for i := 0; i < entriesPerRun; i++ {
					in <- newEntry(nil, maps.Clone(lb), rawTestLine, ts)
				}
				close(in)
				<-done
			}(pl)
		}
		wg.Wait()
	}
	b.ReportMetric(float64(streams*entriesPerRun*b.N)/b.Elapsed().Seconds(), "entries/sec")
}
