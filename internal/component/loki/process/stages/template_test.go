package stages

import (
	"strconv"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/syntax"
)

var testTemplateLogLine = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN",
	"nested" : {"child":"value"},
	"message" : "this is a log line"
}
`

var testTemplateLogLineWithMissingKey = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"component": ["parser","type"],
	"level" : "WARN",
	"nested" : {"child":"value"},
	"message" : "this is a log line"
}
`

func TestUnmarshalTemplateConfig(t *testing.T) {
	type testCase struct {
		name      string
		cfg       string
		expectErr bool
	}

	tests := []testCase{
		{
			name: "valid",
			cfg: `
				source = "test"
				template = "{{.Value}}"
			`,
		},
		{
			name: "missing source",
			cfg: `
				template = "{{.Value}}"
			`,
			expectErr: true,
		},
		{
			name: "invalid template",
			cfg: `
				source = "test"
				template = "{{{.Value}}}"
			`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg TemplateConfig
			err := syntax.Unmarshal([]byte(tt.cfg), &cfg)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTemplateStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "simple template",
			config: `
			stage.template {
				source   = "some"
				template = "{{ .Value }} appended"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"some": "value"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"some": "value appended"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "add missing",
			config: `
			stage.template {
				source   = "missing"
				template = "newval"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"notmissing": "value"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"notmissing": "value", "missing": "newval"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "template with multiple keys",
			config: `
			stage.template {
				source   = "message"
				template = "{{.Value}} in module {{.module}}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"level":   "warn",
					"app":     "loki",
					"message": "warn for app loki",
					"module":  "test",
				}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"level":   "warn",
					"app":     "loki",
					"module":  "test",
					"message": "warn for app loki in module test",
				}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "template with multiple keys with missing source",
			config: `
			stage.template {
				source   = "missing"
				template = "{{ .level }} for app {{ .app | ToUpper }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"level": "warn", "app": "loki"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"level": "warn", "app": "loki", "missing": "warn for app LOKI"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "template with multiple keys with missing key",
			config: `
			stage.template {
				source   = "message"
				template = "{{.Value}} in module {{.module}}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"level":   "warn",
					"app":     "loki",
					"message": "warn for app loki",
				}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"level":   "warn",
					"app":     "loki",
					"message": "warn for app loki in module <no value>",
				}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "template with multiple keys with nil value in extracted key",
			config: `
			stage.template {
				source   = "level"
				template = "{{ Replace .Value \"Warning\" \"warn\" 1 }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"level": "Warning", "testval": nil}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"level": "warn", "testval": nil}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "ToLower",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ .Value | ToLower }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Value"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "value"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "sprig",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ add 7 3 }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Value"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "10"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "ToLowerParams",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ ToLower .Value }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Value"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "value"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "ToLowerEmptyValue",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ .Value | ToLower }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "ReplaceAllToLower",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ Replace .Value \" \" \"_\" -1 | ToLower }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Some Silly Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "some_silly_value_with_lots_of_spaces"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "regexReplaceAll",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ regexReplaceAll \"(Silly)\" .Value \"${1}foo\"  }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Some Silly Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "Some Sillyfoo Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "regexReplaceAllerr",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ regexReplaceAll \"\\\\K\" .Value \"${1}foo\"  }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Some Silly Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "Some Silly Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "regexReplaceAllLiteral",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ regexReplaceAll \"( |Of)\" .Value \"_\"  }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Some Silly Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "Some_Silly_Value_With_Lots___Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "regexReplaceAllLiteralerr",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ regexReplaceAll \"\\\\K\" .Value \"err\"  }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "Some Silly Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "Some Silly Value With Lots Of Spaces"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "Trim",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ Trim .Value \"!\" }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "!!!!!WOOOOO!!!!!"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "WOOOOO"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "Remove label empty value",
			config: `
			stage.template {
				source   = "testval"
				template = ""
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "WOOOOO"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "Don't add label with empty value",
			config: `
			stage.template {
				source   = "testval"
				template = ""
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "Sha2Hash",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ Sha2Hash .Value \"salt\" }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "this is PII data"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "5526fd6f8ad457279cf8ff06453c6cb61bf479fa826e3b099caa6c846f9376f2"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "Hash",
			config: `
			stage.template {
				source   = "testval"
				template = "{{ Hash .Value \"salt\" }}"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"testval": "this is PII data"}, model.LabelSet{}, "not important for this test", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"testval": "0807ea24e992127128b38e4930f7155013786a4999c73a25910318a793847658"}, model.LabelSet{}, "not important for this test", now),
			},
		},
		{
			name: "pipeline with multiple template stages rendering into labels",
			config: `
			stage.json {
				expressions = { "app" = "app", "level" = "level" }
			}
			stage.template {
				source   = "app"
				template = "{{ .Value | ToUpper }} doki"
			}
			stage.template {
				source   = "level"
				template = "{{ if eq .Value \"WARN\" }}{{ Replace .Value \"WARN\" \"OK\" -1 }}{{ else }}{{ .Value }}{{ end }}"
			}
			stage.template {
				source   = "nonexistent"
				template = "TEST"
			}
			stage.labels {
				values = { "app" = "", "level" = "", "type" = "nonexistent" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testTemplateLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"app":         "LOKI doki",
					"level":       "OK",
					"nonexistent": "TEST",
				}, model.LabelSet{
					"app":   "LOKI doki",
					"level": "OK",
					"type":  "TEST",
				}, testTemplateLogLine, now),
			},
		},
		{
			name: "missing extracted source from json expression leaves that key unchanged",
			config: `
			stage.json {
				expressions = { "app" = "app", "level" = "level" }
			}
			stage.template {
				source   = "app"
				template = "{{ .Value | ToUpper }} doki"
			}
			stage.template {
				source   = "level"
				template = "{{ if eq .Value \"WARN\" }}{{ Replace .Value \"WARN\" \"OK\" -1 }}{{ else }}{{ .Value }}{{ end }}"
			}
			stage.template {
				source   = "nonexistent"
				template = "TEST"
			}
			stage.labels {
				values = { "app" = "", "level" = "", "type" = "nonexistent" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testTemplateLogLineWithMissingKey, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"app":         nil,
					"level":       "OK",
					"nonexistent": "TEST",
				}, model.LabelSet{
					"level": "OK",
					"type":  "TEST",
				}, testTemplateLogLineWithMissingKey, now),
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

func BenchmarkTemplateStage(b *testing.B) {
	cfg := TemplateConfig{
		Source:   "1",
		Template: mustTemplate("{{ .Value }}"),
	}

	labels := make(model.LabelSet, 11)
	for i := 0; i <= 10; i++ {
		v := strconv.FormatInt(int64(i), 10)
		labels[model.LabelName(v)] = model.LabelValue(v)
	}

	batch := loki.NewBatch()
	batch.Add(loki.NewStream(labels, push.Entry{
		Timestamp: time.Now(),
	}))

	runPipelineBenchmark(b, []StageConfig{{TemplateConfig: &cfg}}, batch)
}

func mustTemplate(text string) Template {
	t := Template(text)
	err := t.UnmarshalText([]byte(text))
	if err != nil {
		panic(err)
	}
	return t
}
