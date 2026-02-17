package stages

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/regexp"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var protocolStr = "protocol"

var testRegexAlloySingleStageWithoutSource = `
stage.regex {
    expression =  "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
}
`

var testRegexAlloyMultiStageWithSource = `
stage.regex {
    expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
}
stage.regex {
    expression = "^HTTP\\/(?P<protocol_version>[0-9\\.]+)$"
    source     = "protocol"
}
`

var testRegexAlloyMultiStageWithSourceAndLabelFromGroups = `
stage.regex {
    expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
	labels_from_groups = true
}
stage.regex {
    expression = "^HTTP\\/(?P<protocol_version>[0-9\\.]+)$"
    source     = "protocol"
}
`

var testRegexAlloyMultiStageWithExistingLabelsAndLabelFromGroups = `
stage.static_labels {
    values = {
      protocol = "HTTP/2",
    }
}
stage.regex {
    expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
	labels_from_groups = true
}
`

var testRegexAlloySourceWithMissingKey = `
stage.json {
    expressions = { "time" = "" }
}
stage.regex {
    expression = "^(?P<year>\\d+)"
    source     = "time"
}
`

var testRegexLogLineWithMissingKey = `
{
	"app":"loki",
	"component": ["parser","type"],
	"level": "WARN"
}
`

var testRegexLogLine = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`

func init() {
	Debug = true
}

func TestPipeline_Regex(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config          string
		entry           string
		expectedExtract map[string]any
		expectedLables  model.LabelSet
	}{
		"successfully run a pipeline with 1 regex stage without source": {
			testRegexAlloySingleStageWithoutSource,
			testRegexLogLine,
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
		"successfully run a pipeline with 2 regex stages with source": {
			testRegexAlloyMultiStageWithSource,
			testRegexLogLine,
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
		"successfully run a pipeline with 2 regex stages with source and labels from groups": {
			testRegexAlloyMultiStageWithSourceAndLabelFromGroups,
			testRegexLogLine,
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
		"successfully run a pipeline with regex stage labels overriding existing labels with labels_from_groups": {
			testRegexAlloyMultiStageWithExistingLabelsAndLabelFromGroups,
			testRegexLogLine,
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
			pl, err := NewPipeline(logger, loadConfig(testData.config), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				t.Fatal(err)
			}

			out := processEntries(pl, newEntry(nil, nil, testData.entry, time.Now()))[0]
			assert.Equal(t, testData.expectedExtract, out.Extracted)
			assert.Equal(t, testData.expectedLables, out.Labels)
		})
	}
}

func TestPipelineWithMissingKey_Regex(t *testing.T) {
	var buf bytes.Buffer
	w := log.NewSyncWriter(&buf)
	logger := log.NewLogfmtLogger(w)
	pl, err := NewPipeline(logger, loadConfig(testRegexAlloySourceWithMissingKey), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	_ = processEntries(pl, newEntry(nil, nil, testRegexLogLineWithMissingKey, time.Now()))[0]

	expectedLog := "level=debug component=stage type=regex msg=\"failed to convert source value to string\" source=time err=\"can't convert <nil> to string\" type=null"
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

func TestRegexConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name      string
		config    string
		expextErr bool
	}

	tests := []testCase{

		{
			name:      "empty config",
			config:    "",
			expextErr: true,
		},
		{
			name:      "missing expression",
			config:    "",
			expextErr: true,
		},
		{
			name: "invalid expression",
			config: `
				expression = "(?P<ts[0-9]+).*"
			`,
			expextErr: true,
		},
		{
			name: "empty source",
			config: `
				expression = "(?P<ts>[0-9]+).*"
				source = ""
			`,
			expextErr: true,
		},
		{
			name: "valid without source",
			config: `
				expression = "(?P<ts>[0-9]+).*"
			`,
		},
		{
			name: "valid with source",
			config: `
				expression = "(?P<ts>[0-9]+).*"
				source     = "log"
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg RegexConfig
			err := syntax.Unmarshal([]byte(tt.config), &cfg)

			if tt.expextErr {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}
		})
	}
}

var regexLogFixture = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`

func TestRegexParser_Parse(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		config          RegexConfig
		extracted       map[string]any
		labels          model.LabelSet
		entry           string
		expectedExtract map[string]any
		expectedLabels  model.LabelSet
	}{
		"successfully match expression on entry": {
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"),
			},
			map[string]any{},
			model.LabelSet{},
			regexLogFixture,
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
			RegexConfig{
				Expression:       regexp.MustCompileNonEmpty("^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"),
				LabelsFromGroups: true,
			},
			map[string]any{},
			model.LabelSet{},
			regexLogFixture,
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
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("^HTTP\\/(?P<protocol_version>.*)$"),
				Source:     &protocolStr,
			},
			map[string]any{
				"protocol": "HTTP/1.1",
			},
			model.LabelSet{},
			regexLogFixture,
			map[string]any{
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
			},
			model.LabelSet{},
		},
		"successfully match expression on extracted[source] with label extracted from named capture groups": {
			RegexConfig{
				Expression:       regexp.MustCompileNonEmpty("^HTTP\\/(?P<protocol_version>.*)$"),
				Source:           &protocolStr,
				LabelsFromGroups: true,
			},
			map[string]any{
				"protocol": "HTTP/1.1",
			},
			model.LabelSet{
				"protocol": "HTTP/1.1",
			},
			regexLogFixture,
			map[string]any{
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
			},
			model.LabelSet{
				"protocol":         "HTTP/1.1",
				"protocol_version": "1.1",
			},
		},
		"failed to match expression on entry": {
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<flags>\\S+?) (?P<message>.*)$"),
			},
			map[string]any{},
			model.LabelSet{},
			"blahblahblah",
			map[string]any{},
			model.LabelSet{},
		},
		"failed to match expression on extracted[source]": {
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("^HTTP\\/(?P<protocol_version>.*)$"),
				Source:     &protocolStr,
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
		"case insensitive": {
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("(?i)(?P<bad>panic:|core_dumped|failure|error|attack| bad |illegal |denied|refused|unauthorized|fatal|failed|Segmentation Fault|Corrupted)"),
			},
			map[string]any{},
			model.LabelSet{},
			"A Terrible Error has occurred!!!",
			map[string]any{
				"bad": "Error",
			},
			model.LabelSet{},
		},
		"missing extracted[source]": {
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("^HTTP\\/(?P<protocol_version>.*)$"),
				Source:     &protocolStr,
			},
			map[string]any{},
			model.LabelSet{},
			"blahblahblah",
			map[string]any{},
			model.LabelSet{},
		},
		"invalid data type in extracted[source]": {
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("^HTTP\\/(?P<protocol_version>.*)$"),
				Source:     &protocolStr,
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
			p, err := New(logger, StageConfig{RegexConfig: &tt.config}, nil, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				t.Fatalf("failed to create regex parser: %s", err)
			}
			out := processEntries(p, newEntry(tt.extracted, tt.labels, tt.entry, time.Now()))[0]
			assert.Equal(t, tt.expectedExtract, out.Extracted)
			assert.Equal(t, tt.expectedLabels, out.Labels)
		})
	}
}

func BenchmarkRegexStage(b *testing.B) {
	benchmarks := []struct {
		name   string
		config RegexConfig
		entry  string
	}{
		{"apache common log",
			RegexConfig{
				Expression: regexp.MustCompileNonEmpty("^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"),
			},
			regexLogFixture,
		},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			logger := util.TestAlloyLogger(b)
			stage, err := New(logger, StageConfig{RegexConfig: &bm.config}, nil, featuregate.StabilityGenerallyAvailable)
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
