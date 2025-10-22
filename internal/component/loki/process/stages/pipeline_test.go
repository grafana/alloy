package stages

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/loki/pkg/push"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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

func newPipelineFromConfig(cfg, name string) (*Pipeline, error) {
	return NewPipeline(log.NewNopLogger(), loadConfig(cfg), &name, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
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
	p, err := NewPipeline(log.NewNopLogger(), loadConfig(testMultiStageAlloy), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
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

			p, err := NewPipeline(log.NewNopLogger(), loadConfig(tt.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
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
	l           = log.NewNopLogger()
	infoLogger  = level.NewFilter(l, level.AllowInfo())
	debugLogger = level.NewFilter(l, level.AllowDebug())
)

func BenchmarkPipeline(b *testing.B) {
	benchmarks := []struct {
		name   string
		stgs   []StageConfig
		logger log.Logger
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
			pl, err := NewPipeline(bm.logger, bm.stgs, nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
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
			for i := 0; i < b.N; i++ {
				in <- newEntry(nil, lb, bm.entry, ts)
			}
			close(in)
		})
	}
}

func TestPipeline_Wrap(t *testing.T) {
	now := time.Now()
	p, err := NewPipeline(log.NewNopLogger(), loadConfig(testMultiStageAlloy), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
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
		tt := tt
		t.Run(tName, func(t *testing.T) {
			t.Parallel()
			c := fake.NewClient(func() {})
			handler := p.Wrap(c)

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
	c := fake.NewClient(func() {})
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
	p, err := newPipelineFromConfig(cfg, "test")
	require.NoError(t, err)

	e1 := p.Wrap(c)
	e2 := loki.AddLabelsMiddleware(model.LabelSet{"bar": "foo"}).Wrap(e1)
	entryhandler := loki.AddLabelsMiddleware(model.LabelSet{"foo": "bar"}).Wrap(e2)

	var wg sync.WaitGroup
	parallelism := 10
	wg.Add(parallelism)

	for i := 0; i < parallelism; i++ {
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
	c.Stop()
	t.Log(c.Received())
}
