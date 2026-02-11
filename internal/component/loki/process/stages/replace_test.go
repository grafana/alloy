package stages

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var testReplaceAlloySingleStageWithoutSource = `
stage.replace {
		expression = "11.11.11.11 - (\\S+) .*"
		replace    = "dummy"
}
`
var testReplaceAlloyMultiStageWithSource = `
stage.json {
		expressions = { "level" = "", "msg" = "" }
}

stage.replace {
		expression = "\\S+ - \"POST (\\S+) .*"
    	source     = "msg"
    	replace    = "/loki/api/v1/push/"
}
`

var testReplaceAlloyWithNamedCapturedGroupWithTemplate = `
stage.replace {
		expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
		replace    = "{{ if eq .Value \"200\" }}{{ Replace .Value \"200\" \"HttpStatusOk\" -1 }}{{ else }}{{ .Value | ToUpper }}{{ end }}"
}
`

var testReplaceAlloyWithNestedCapturedGroups = `
stage.replace {
		expression = "(?P<ip_user>^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+)) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action_path>(?P<action>\\S+)\\s?(?P<path>\\S+)?)\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
		replace    = "{{ if eq .Value \"200\" }}{{ Replace .Value \"200\" \"HttpStatusOk\" -1 }}{{ else }}{{ .Value | ToUpper }}{{ end }}"
}
`

var testReplaceAlloyWithTemplate = `
stage.replace {
		expression = "^(\\S+) (\\S+) (\\S+) \\[([\\w:/]+\\s[+\\-]\\d{4})\\] \"(\\S+)\\s?(\\S+)?\\s?(\\S+)?\" (\\d{3}|-) (\\d+|-)\\s?\"?([^\"]*)\"?\\s?\"?([^\"]*)?\"?$"
		replace    = "{{ if eq .Value \"200\" }}{{ Replace .Value \"200\" \"HttpStatusOk\" -1 }}{{ else }}{{ .Value | ToUpper }}{{ end }}"
}
`

var testReplaceAlloyWithEmptyReplace = `
stage.replace {
		expression = "11.11.11.11 - (\\S+\\s)"
		replace    = ""
}
`

var testReplaceAdjacentCaptureGroups = `
stage.replace {
		expression = "(a|b|c)"
		replace    = ""
}
`

var testReplaceLogLine = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`
var testReplaceLogJSONLine = `{"time":"2019-01-01T01:00:00.000000001Z", "level": "info", "msg": "11.11.11.11 - \"POST /loki/api/push/ HTTP/1.1\" 200 932 \"-\" \"Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6\""}`
var testReplaceLogLineAdjacentCaptureGroups = `abc`

func TestReplace(t *testing.T) {
	t.Parallel()
	logger := util.TestAlloyLogger(t)

	tests := map[string]struct {
		config        string
		entry         string
		extracted     map[string]any
		expectedEntry string
	}{
		"successfully run a pipeline with 1 regex stage without source": {
			testReplaceAlloySingleStageWithoutSource,
			testReplaceLogLine,
			map[string]any{},
			`11.11.11.11 - dummy [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`,
		},
		"successfully run a pipeline with multi stage with": {
			testReplaceAlloyMultiStageWithSource,
			testReplaceLogJSONLine,
			map[string]any{
				"level": "info",
				"msg":   `11.11.11.11 - "POST /loki/api/v1/push/ HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`,
			},
			`{"time":"2019-01-01T01:00:00.000000001Z", "level": "info", "msg": "11.11.11.11 - \"POST /loki/api/push/ HTTP/1.1\" 200 932 \"-\" \"Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6\""}`,
		},
		"successfully run a pipeline with 1 regex stage with named captured group and with template and without source": {
			testReplaceAlloyWithNamedCapturedGroupWithTemplate,
			testReplaceLogLine,
			map[string]any{
				"ip":        "11.11.11.11",
				"identd":    "-",
				"user":      "FRANK",
				"timestamp": "25/JAN/2000:14:00:01 -0500",
				"action":    "GET",
				"path":      "/1986.JS",
				"protocol":  "HTTP/1.1",
				"status":    "HttpStatusOk",
				"referer":   "-",
				"useragent": "MOZILLA/5.0 (WINDOWS; U; WINDOWS NT 5.1; DE; RV:1.9.1.7) GECKO/20091221 FIREFOX/3.5.7 GTB6",
			},
			`11.11.11.11 - FRANK [25/JAN/2000:14:00:01 -0500] "GET /1986.JS HTTP/1.1" HttpStatusOk 932 "-" "MOZILLA/5.0 (WINDOWS; U; WINDOWS NT 5.1; DE; RV:1.9.1.7) GECKO/20091221 FIREFOX/3.5.7 GTB6"`,
		},
		"successfully run a pipeline with 1 regex stage with nested captured groups and with template and without source": {
			testReplaceAlloyWithNestedCapturedGroups,
			testReplaceLogLine,
			map[string]any{
				"ip_user":     "11.11.11.11 - FRANK",
				"action_path": "GET /1986.JS",
				"ip":          "11.11.11.11",
				"identd":      "-",
				"user":        "FRANK",
				"timestamp":   "25/JAN/2000:14:00:01 -0500",
				"action":      "GET",
				"path":        "/1986.JS",
				"protocol":    "HTTP/1.1",
				"status":      "HttpStatusOk",
				"referer":     "-",
				"useragent":   "MOZILLA/5.0 (WINDOWS; U; WINDOWS NT 5.1; DE; RV:1.9.1.7) GECKO/20091221 FIREFOX/3.5.7 GTB6",
			},
			`11.11.11.11 - FRANK [25/JAN/2000:14:00:01 -0500] "GET /1986.JS HTTP/1.1" HttpStatusOk 932 "-" "MOZILLA/5.0 (WINDOWS; U; WINDOWS NT 5.1; DE; RV:1.9.1.7) GECKO/20091221 FIREFOX/3.5.7 GTB6"`,
		},
		"successfully run a pipeline with 1 regex stage with template and without source": {
			testReplaceAlloyWithTemplate,
			testReplaceLogLine,
			map[string]any{},
			`11.11.11.11 - FRANK [25/JAN/2000:14:00:01 -0500] "GET /1986.JS HTTP/1.1" HttpStatusOk 932 "-" "MOZILLA/5.0 (WINDOWS; U; WINDOWS NT 5.1; DE; RV:1.9.1.7) GECKO/20091221 FIREFOX/3.5.7 GTB6"`,
		},
		"successfully run a pipeline with empty replace value": {
			testReplaceAlloyWithEmptyReplace,
			testReplaceLogLine,
			map[string]any{},
			`11.11.11.11 - [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`,
		},
		"successfully run a pipeline with adjacent capture groups": {
			testReplaceAdjacentCaptureGroups,
			testReplaceLogLineAdjacentCaptureGroups,
			map[string]any{},
			``,
		},
	}

	for testName, testData := range tests {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(logger, loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				t.Fatal(err)
			}
			out := processEntries(pl, newEntry(nil, nil, testData.entry, time.Now()))[0]
			assert.Equal(t, testData.expectedEntry, out.Line)
			assert.Equal(t, testData.extracted, out.Extracted)
		})
	}
}

func TestReplaceConfigValidation(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		config ReplaceConfig
		err    error
	}{
		"missing regex_expression": {
			ReplaceConfig{},
			ErrExpressionRequired,
		},
		"invalid regex_expression": {
			ReplaceConfig{
				Expression: "(?P<ts[0-9]+).*",
				Replace:    "test",
			},
			fmt.Errorf("%v: %w", ErrCouldNotCompileRegex, errors.New("error parsing regexp: invalid named capture: `(?P<ts[0-9]+).*`")),
		},
		"valid without source": {
			ReplaceConfig{
				Expression: "(?P<ts>[0-9]+).*",
				Replace:    "test",
			},
			nil,
		},
		"valid with source": {
			ReplaceConfig{
				Expression: "(?P<ts>[0-9]+).*",
				Source:     "log",
				Replace:    "test",
			},
			nil,
		},
	}
	for tName, tt := range tests {
		tt := tt
		t.Run(tName, func(t *testing.T) {
			_, err := getExpressionRegex(tt.config)
			if (err != nil) != (tt.err != nil) {
				t.Errorf("ReplaceConfig.validate() expected error = %v, actual error = %v", tt.err, err)
				return
			}
			if (err != nil) && (err.Error() != tt.err.Error()) {
				t.Errorf("ReplaceConfig.validate() expected error = %v, actual error = %v", tt.err, err)
				return
			}
		})
	}
}
