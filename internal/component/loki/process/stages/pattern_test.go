package stages

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

var testPatternAlloySingleStageWithoutSource = `
stage.pattern {
    pattern = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
}
`

var testPatternAlloyMultiStageWithSource = `
stage.pattern {
    pattern = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
}
stage.pattern {
    pattern    = "HTTP/<protocol_version>"
    source     = "protocol"
}
`

// "^HTTP\\/(?P<protocol_version>[0-9\\.]+)$"
var testPatternAlloyMultiStageWithSourceAndLabelFromGroups = `
stage.pattern {
    pattern 	       = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
	labels_from_groups = true
}
stage.pattern {
    pattern    = "HTTP/<protocol_version>"
    source     = "protocol"
}
`

var testPatternAlloyMultiStageWithExistingLabelsAndLabelFromGroups = `
stage.static_labels {
    values = {
      protocol = "HTTP/2",
    }
}
stage.pattern {
    pattern = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
	labels_from_groups = true
}
`

var testPatternAlloySourceWithMissingKey = `
stage.json {
    expressions = { "time" = "" }
}
stage.pattern {
    pattern = "<year>/<month>/<day>"
    source  = "time"
}
`

var testPatternLogLineWithMissingKey = `
{
	"app":"loki",
	"component": ["parser","type"],
	"level": "WARN"
}
`

var testPatternLogLine = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`

func init() {
	Debug = true
}

func TestPipeline_Pattern(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config          string
		entry           string
		expectedExtract map[string]any
		expectedLables  model.LabelSet
	}{
		"successfully run a pipeline with 1 pattern stage without source": {
			testPatternAlloySingleStageWithoutSource,
			testPatternLogLine,
			map[string]any{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "frank",
				"timestamp": "25/Jan/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.js",
				"protocol":  "HTTP/1.1",
				"status":    "200",
				"size":      "932",
				"referer":   "-",
				"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
			model.LabelSet{},
		},
		"successfully run a pipeline with 2 pattern stages with source": {
			testPatternAlloyMultiStageWithSource,
			testPatternLogLine,
			map[string]any{
				"ip":               "11.11.11.11",
				"identd":           "-",
				"user":             "frank",
				"timestamp":        "25/Jan/2000:14:00:01 -0500",
				"action":           "GET",
				"path":             "/1986.js",
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
				"status":           "200",
				"size":             "932",
				"referer":          "-",
				"useragent":        "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
			model.LabelSet{},
		},
		"successfully run a pipeline with 2 pattern stages with source and labels from groups": {
			testPatternAlloyMultiStageWithSourceAndLabelFromGroups,
			testPatternLogLine,
			map[string]any{
				"ip":               "11.11.11.11",
				"identd":           "-",
				"user":             "frank",
				"timestamp":        "25/Jan/2000:14:00:01 -0500",
				"action":           "GET",
				"path":             "/1986.js",
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
				"status":           "200",
				"size":             "932",
				"referer":          "-",
				"useragent":        "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
			model.LabelSet{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "frank",
				"timestamp": "25/Jan/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.js",
				"protocol":  "HTTP/1.1",
				"status":    "200",
				"size":      "932",
				"referer":   "-",
				"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
		},
		"successfully run a pipeline with pattern stage labels overriding existing labels with labels_from_groups": {
			testPatternAlloyMultiStageWithExistingLabelsAndLabelFromGroups,
			testPatternLogLine,
			map[string]any{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "frank",
				"timestamp": "25/Jan/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.js",
				"protocol":  "HTTP/1.1",
				"status":    "200",
				"size":      "932",
				"referer":   "-",
				"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
			model.LabelSet{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "frank",
				"timestamp": "25/Jan/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.js",
				"protocol":  "HTTP/1.1",
				"status":    "200",
				"size":      "932",
				"referer":   "-",
				"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
		},
	}

	for testName, testData := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			logger := util.TestAlloyLogger(t)
			pl, err := NewPipeline(logger, loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				t.Fatal(err)
			}

			out := processEntries(pl, newEntry(nil, nil, testData.entry, time.Now()))[0]
			assert.Equal(t, testData.expectedExtract, out.Extracted)
			assert.Equal(t, testData.expectedLables, out.Labels)
		})
	}
}

func TestPipelineWithMissingKey_Pattern(t *testing.T) {
	var buf bytes.Buffer
	w := log.NewSyncWriter(&buf)
	logger := log.NewLogfmtLogger(w)
	pl, err := NewPipeline(logger, loadConfig(testPatternAlloySourceWithMissingKey), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	_ = processEntries(pl, newEntry(nil, nil, testPatternLogLineWithMissingKey, time.Now()))[0]

	expectedLog := "level=debug component=stage type=pattern msg=\"failed to convert source value to string\" source=time err=\"can't convert <nil> to string\" type=null"
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

func TestPatternConfig_validate(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		config any
		err    error
	}{
		"empty config": {
			nil,
			ErrPatternRequired,
		},
		"missing pattern_expression": {
			map[string]any{},
			ErrPatternRequired,
		},
		"invalid pattern_expression": {
			map[string]any{
				"pattern": "<_> <_>",
			},
			errors.New("failed to parse pattern: at least one capture is required"),
		},
		"empty source": {
			map[string]any{
				"pattern": "(?P<ts>[0-9]+).*",
				"source":  "",
			},
			ErrEmptyPatternStageSource,
		},
		"valid without source": {
			map[string]any{
				"pattern": "(?P<ts>[0-9]+).*",
			},
			nil,
		},
		"valid with source": {
			map[string]any{
				"pattern": "(?P<ts>[0-9]+).*",
				"source":  "log",
			},
			nil,
		},
	}
	for tName, tt := range tests {
		tt := tt
		t.Run(tName, func(t *testing.T) {
			c, err := parsePatternConfig(tt.config)
			if err != nil {
				t.Fatalf("failed to create config: %s", err)
			}
			_, err = validatePatternConfig(*c)
			if (err != nil) != (tt.err != nil) {
				t.Errorf("PatternConfig.validate() expected error = %v, actual error = %v", tt.err, err)
				return
			}
			if (err != nil) && (err.Error() != tt.err.Error()) {
				t.Errorf("PatternConfig.validate() expected error = %v, actual error = %v", tt.err, err)
				return
			}
		})
	}
}

var patternLogFixture = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`

func TestPatternParser_Parse(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		config          PatternConfig
		extracted       map[string]any
		labels          model.LabelSet
		entry           string
		expectedExtract map[string]any
		expectedLabels  model.LabelSet
	}{
		"successfully match expression on entry": {
			PatternConfig{
				Pattern: "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\"",
			},
			map[string]any{},
			model.LabelSet{},
			patternLogFixture,
			map[string]any{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "frank",
				"timestamp": "25/Jan/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.js",
				"protocol":  "HTTP/1.1",
				"status":    "200",
				"size":      "932",
				"referer":   "-",
				"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
			model.LabelSet{},
		},
		"successfully match expression on entry with label extracted from named capture groups": {
			PatternConfig{
				Pattern:          "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\"",
				LabelsFromGroups: true,
			},
			map[string]any{},
			model.LabelSet{},
			patternLogFixture,
			map[string]any{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "frank",
				"timestamp": "25/Jan/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.js",
				"protocol":  "HTTP/1.1",
				"status":    "200",
				"size":      "932",
				"referer":   "-",
				"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
			model.LabelSet{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "frank",
				"timestamp": "25/Jan/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.js",
				"protocol":  "HTTP/1.1",
				"status":    "200",
				"size":      "932",
				"referer":   "-",
				"useragent": "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6",
			},
		},
		"successfully match expression on extracted[source]": {
			PatternConfig{
				Pattern: "HTTP/<protocol_version>",
				Source:  &protocolStr,
			},
			map[string]any{
				"protocol": "HTTP/1.1",
			},
			model.LabelSet{},
			patternLogFixture,
			map[string]any{
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
			},
			model.LabelSet{},
		},
		"successfully match expression on extracted[source] with label extracted from named capture groups": {
			PatternConfig{
				Pattern:          "HTTP/<protocol_version>",
				Source:           &protocolStr,
				LabelsFromGroups: true,
			},
			map[string]any{
				"protocol": "HTTP/1.1",
			},
			model.LabelSet{
				"protocol": "HTTP/1.1",
			},
			patternLogFixture,
			map[string]any{
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
			},
			model.LabelSet{
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
			},
		},
		"match a message that is not quoted": {
			PatternConfig{
				Pattern: "<time> <stream> <flags> <message>",
			},
			map[string]any{},
			model.LabelSet{},
			"2019-01-01T01:00:00.000000001Z stderr P i'm a log message!",
			map[string]any{
				"time":    "2019-01-01T01:00:00.000000001Z",
				"stream":  "stderr",
				"flags":   "P",
				"message": "i'm a log message!",
			},
			model.LabelSet{},
		},
		"failed to match expression on extracted[source]": {
			PatternConfig{
				Pattern: "HTTP/<protocol_version>",
				Source:  &protocolStr,
			},
			map[string]any{
				"protocol": "unknown",
			},
			model.LabelSet{},
			"unknown/unknown",
			map[string]any{
				"protocol": "unknown",
			},
			model.LabelSet{},
		},
		"missing extracted[source]": {
			PatternConfig{
				Pattern: "HTTP/<protocol_version>",
				Source:  &protocolStr,
			},
			map[string]any{},
			model.LabelSet{},
			"blahblahblah",
			map[string]any{},
			model.LabelSet{},
		},
		"invalid data type in extracted[source]": {
			PatternConfig{
				Pattern: "HTTP/<protocol_version>",
				Source:  &protocolStr,
			},
			map[string]any{
				"protocol": true,
			},
			model.LabelSet{},
			"unknown/unknown",
			map[string]any{
				"protocol": true,
			},
			model.LabelSet{},
		},
	}
	for tName, tt := range tests {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()
			logger := util.TestAlloyLogger(t)
			p, err := New(logger, nil, StageConfig{PatternConfig: &tt.config}, nil, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				t.Fatalf("failed to create pattern parser: %s", err)
			}
			out := processEntries(p, newEntry(tt.extracted, tt.labels, tt.entry, time.Now()))[0]
			assert.Equal(t, tt.expectedExtract, out.Extracted)
			assert.Equal(t, tt.expectedLabels, out.Labels)
		})
	}
}

func BenchmarkPatternStage(b *testing.B) {
	benchmarks := []struct {
		name   string
		config PatternConfig
		entry  string
	}{
		{"apache common log",
			PatternConfig{
				Pattern: "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\"",
			},
			patternLogFixture,
		},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			logger := util.TestAlloyLogger(b)
			stage, err := New(logger, nil, StageConfig{PatternConfig: &bm.config}, nil, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				panic(err)
			}
			labels := model.LabelSet{}
			ts := time.Now()
			extr := map[string]any{}

			in := make(chan Entry)
			out := stage.Run(in)
			go func() {
				for range out {
				}
			}()
			for i := 0; i < b.N; i++ {
				in <- newEntry(extr, labels, bm.entry, ts)
			}
			close(in)
		})
	}
}
