package stages

import (
	"errors"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

var (
	regexLogFixture                = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`
	testRegexLogLineWithMissingKey = `
	{
		"app":"loki",
		"component": ["parser","type"],
		"level": "WARN"
	}
	`
)

func TestRegexStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "successfully match expression on entry",
			config: `
			stage.regex {
				expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
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
				}, model.LabelSet{}, regexLogFixture, now),
			},
		},
		{
			name: "successfully match expression on entry with label extracted from named capture groups",
			config: `
			stage.regex {
				expression         = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
				labels_from_groups = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
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
				}, model.LabelSet{
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
				}, regexLogFixture, now),
			},
		},
		{
			name: "successfully match expression on extracted source",
			config: `
			stage.regex {
				expression = "^HTTP\\/(?P<protocol_version>.*)$"
				source     = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"protocol": "HTTP/1.1",
				}, model.LabelSet{}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"protocol":         "HTTP/1.1",
					"protocol_version": "1.1",
				}, model.LabelSet{}, regexLogFixture, now),
			},
		},
		{
			name: "successfully match expression on extracted source with label extracted from named capture groups",
			config: `
			stage.regex {
				expression         = "^HTTP\\/(?P<protocol_version>.*)$"
				source             = "protocol"
				labels_from_groups = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"protocol": "HTTP/1.1",
				}, model.LabelSet{
					"protocol": "HTTP/1.1",
				}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"protocol":         "HTTP/1.1",
					"protocol_version": "1.1",
				}, model.LabelSet{
					"protocol":         "HTTP/1.1",
					"protocol_version": "1.1",
				}, regexLogFixture, now),
			},
		},
		{
			name: "failed to match expression on entry",
			config: `
			stage.regex {
				expression = "^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<flags>\\S+?) (?P<message>.*)$"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "blahblahblah", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "blahblahblah", now),
			},
		},
		{
			name: "failed to match expression on extracted source",
			config: `
			stage.regex {
				expression = "^HTTP\\/(?P<protocol_version>.*)$"
				source     = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"protocol": "unknown",
				}, model.LabelSet{}, "unknown/unknown", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"protocol": "unknown",
				}, model.LabelSet{}, "unknown/unknown", now),
			},
		},
		{
			name: "case insensitive",
			config: `
			stage.regex {
				expression = "(?i)(?P<bad>panic:|core_dumped|failure|error|attack| bad |illegal |denied|refused|unauthorized|fatal|failed|Segmentation Fault|Corrupted)"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "A Terrible Error has occurred!!!", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"bad": "Error",
				}, model.LabelSet{}, "A Terrible Error has occurred!!!", now),
			},
		},
		{
			name: "missing extracted source",
			config: `
			stage.regex {
				expression = "^HTTP\\/(?P<protocol_version>.*)$"
				source     = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "blahblahblah", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "blahblahblah", now),
			},
		},
		{
			name: "invalid data type in extracted source",
			config: `
			stage.regex {
				expression = "^HTTP\\/(?P<protocol_version>.*)$"
				source     = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"protocol": true,
				}, model.LabelSet{}, "unknown/unknown", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"protocol": true,
				}, model.LabelSet{}, "unknown/unknown", now),
			},
		},
		{
			name: "pipeline with 1 regex stage without source",
			config: `
			stage.regex {
				expression =  "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
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
				}, model.LabelSet{}, regexLogFixture, now),
			},
		},
		{
			name: "pipeline with 2 regex stages with source",
			config: `
			stage.regex {
				expression = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
			}
			stage.regex {
				expression = "^HTTP\\/(?P<protocol_version>[0-9\\.]+)$"
				source     = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
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
				}, model.LabelSet{}, regexLogFixture, now),
			},
		},
		{
			name: "pipeline with 2 regex stages with source and labels from groups",
			config: `
			stage.regex {
				expression         = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
				labels_from_groups = true
			}
			stage.regex {
				expression = "^HTTP\\/(?P<protocol_version>[0-9\\.]+)$"
				source     = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
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
				}, model.LabelSet{
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
				}, regexLogFixture, now),
			},
		},
		{
			name: "pipeline with regex stage labels overriding existing labels with labels_from_groups",
			config: `
			stage.static_labels {
				values = { protocol = "HTTP/2" }
			}
			stage.regex {
				expression         = "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$"
				labels_from_groups = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, regexLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
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
				}, model.LabelSet{
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
				}, regexLogFixture, now),
			},
		},
		{
			name: "missing extracted source from json expression leaves entry unchanged",
			config: `
			stage.json {
				expressions = { "time" = "" }
			}
			stage.regex {
				expression = "^(?P<year>\\d+)"
				source     = "time"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testRegexLogLineWithMissingKey, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": nil}, model.LabelSet{}, testRegexLogLineWithMissingKey, now),
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

func TestValidateRegexConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		cfg  RegexConfig
		err  error
	}

	tests := []testCase{
		{
			name: "empty config",
			cfg:  RegexConfig{},
			err:  errExpressionRequired,
		},
		{
			name: "missing regex_expression",
			cfg:  RegexConfig{},
			err:  errExpressionRequired,
		},
		{
			name: "invalid regex_expression",
			cfg:  RegexConfig{Expression: "(?P<ts[0-9]+).*"},
			err:  errors.New(errCouldNotCompileRegex.Error() + ": error parsing regexp: invalid named capture: `(?P<ts[0-9]+).*`"),
		},
		{
			name: "empty source",
			cfg: RegexConfig{
				Expression: "(?P<ts>[0-9]+).*",
				Source:     ptr(""),
			},
			err: errEmptyRegexStageSource,
		},
		{
			name: "valid without source",
			cfg: RegexConfig{
				Expression: "(?P<ts>[0-9]+).*",
			},
		},
		{
			name: "valid with source",
			cfg: RegexConfig{
				Expression: "(?P<ts>[0-9]+).*",
				Source:     ptr("log"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateRegexConfig(tt.cfg)
			if tt.err == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func BenchmarkRegexStage(b *testing.B) {
	cfg := RegexConfig{
		Expression: "^(?P<ip>\\S+) (?P<identd>\\S+) (?P<user>\\S+) \\[(?P<timestamp>[\\w:/]+\\s[+\\-]\\d{4})\\] \"(?P<action>\\S+)\\s?(?P<path>\\S+)?\\s?(?P<protocol>\\S+)?\" (?P<status>\\d{3}|-) (?P<size>\\d+|-)\\s?\"?(?P<referer>[^\"]*)\"?\\s?\"?(?P<useragent>[^\"]*)?\"?$",
	}

	batch := loki.NewBatch()
	batch.Add(loki.NewStream(model.LabelSet{}, push.Entry{
		Timestamp: time.Now(),
		Line:      regexLogFixture,
	}))

	runPipelineBenchmark(b, []StageConfig{{RegexConfig: &cfg}}, batch)
}
