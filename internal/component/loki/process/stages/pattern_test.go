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

func TestValidatePatternConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		cfg  PatternConfig
		err  error
	}

	tests := []testCase{
		{
			name: "empty config",
			cfg:  PatternConfig{},
			err:  errPatternRequired,
		},
		{
			name: "missing pattern_expression",
			cfg:  PatternConfig{},
			err:  errPatternRequired,
		},
		{
			name: "invalid pattern_expression",
			cfg:  PatternConfig{Pattern: "<_> <_>"},
			err:  errors.New("failed to parse pattern: at least one capture is required"),
		},
		{
			name: "empty source",
			cfg: PatternConfig{
				Pattern: "(?P<ts>[0-9]+).*",
				Source:  ptr(""),
			},
			err: errEmptyPatternStageSource,
		},
		{
			name: "valid without source",
			cfg: PatternConfig{
				Pattern: "(?P<ts>[0-9]+).*",
			},
		},
		{
			name: "valid with source",
			cfg: PatternConfig{
				Pattern: "(?P<ts>[0-9]+).*",
				Source:  ptr("log"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePatternConfig(tt.cfg)
			if tt.err == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

var patternLogFixture = `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932 "-" "Mozilla/5.0 (Windows; U; Windows NT 5.1; de; rv:1.9.1.7) Gecko/20091221 Firefox/3.5.7 GTB6"`

func TestPatternStage(t *testing.T) {
	var (
		now                              = time.Now()
		testPatternLogLineWithMissingKey = `
		{
			"app":"loki",
			"component": ["parser","type"],
			"level": "WARN"
		}`
	)

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
			stage.pattern {
				pattern = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, patternLogFixture, now),
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
				}, model.LabelSet{}, patternLogFixture, now),
			},
		},
		{
			name: "successfully match expression on entry with label extracted from named capture groups",
			config: `
			stage.pattern {
				pattern            = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
				labels_from_groups = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, patternLogFixture, now),
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
				}, patternLogFixture, now),
			},
		},
		{
			name: "successfully match expression on extracted source",
			config: `
			stage.pattern {
				pattern = "HTTP/<protocol_version>"
				source  = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"protocol": "HTTP/1.1",
				}, model.LabelSet{}, patternLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"protocol":         "HTTP/1.1",
					"protocol_version": "1.1",
				}, model.LabelSet{}, patternLogFixture, now),
			},
		},
		{
			name: "successfully match expression on extracted source with label extracted from named capture groups",
			config: `
			stage.pattern {
				pattern            = "HTTP/<protocol_version>"
				source             = "protocol"
				labels_from_groups = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"protocol": "HTTP/1.1",
				}, model.LabelSet{
					"protocol": "HTTP/1.1",
				}, patternLogFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"protocol":         "HTTP/1.1",
					"protocol_version": "1.1",
				}, model.LabelSet{
					"protocol":         "HTTP/1.1",
					"protocol_version": "1.1",
				}, patternLogFixture, now),
			},
		},
		{
			name: "match a message that is not quoted",
			config: `
			stage.pattern {
				pattern = "<time> <stream> <flags> <message>"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "2019-01-01T01:00:00.000000001Z stderr P i'm a log message!", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"time":    "2019-01-01T01:00:00.000000001Z",
					"stream":  "stderr",
					"flags":   "P",
					"message": "i'm a log message!",
				}, model.LabelSet{}, "2019-01-01T01:00:00.000000001Z stderr P i'm a log message!", now),
			},
		},
		{
			name: "failed to match expression on extracted source",
			config: `
			stage.pattern {
				pattern = "HTTP/<protocol_version>"
				source  = "protocol"
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
			name: "missing extracted source",
			config: `
			stage.pattern {
				pattern = "HTTP/<protocol_version>"
				source  = "protocol"
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
			stage.pattern {
				pattern = "HTTP/<protocol_version>"
				source  = "protocol"
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
			name: "pipeline with 1 pattern stage without source",
			config: `
			stage.pattern {
				pattern = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, patternLogFixture, now),
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
				}, model.LabelSet{}, patternLogFixture, now),
			},
		},
		{
			name: "pipeline with 2 pattern stages with source",
			config: `
			stage.pattern {
				pattern = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
			}
			stage.pattern {
				pattern = "HTTP/<protocol_version>"
				source  = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, patternLogFixture, now),
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
				}, model.LabelSet{}, patternLogFixture, now),
			},
		},
		{
			name: "pipeline with 2 pattern stages with source and labels from groups",
			config: `
			stage.pattern {
				pattern            = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
				labels_from_groups = true
			}
			stage.pattern {
				pattern = "HTTP/<protocol_version>"
				source  = "protocol"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, patternLogFixture, now),
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
				}, patternLogFixture, now),
			},
		},
		{
			name: "pipeline with pattern stage labels overriding existing labels with labels_from_groups",
			config: `
			stage.static_labels {
				values = { protocol = "HTTP/2" }
			}
			stage.pattern {
				pattern            = "<ip> <identd> <user> [<timestamp>] \"<action> <path> <protocol>\" <status> <size> \"<referer>\" \"<useragent>\""
				labels_from_groups = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, patternLogFixture, now),
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
				}, patternLogFixture, now),
			},
		},
		{
			name: "missing extracted source from json expression leaves entry unchanged",
			config: `
			stage.json {
				expressions = { "time" = "" }
			}
			stage.pattern {
				pattern = "<year>/<month>/<day>"
				source  = "time"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testPatternLogLineWithMissingKey, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"time": nil}, model.LabelSet{}, testPatternLogLineWithMissingKey, now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected, "")
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
			batch := loki.NewBatch()
			batch.Add(loki.NewStream(model.LabelSet{}, push.Entry{
				Timestamp: time.Now(),
				Line:      bm.entry,
			}))
			runPipelineBenchmark(b, []StageConfig{{PatternConfig: &bm.config}}, batch)
		})
	}
}
