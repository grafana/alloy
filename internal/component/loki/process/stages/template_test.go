package stages

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

var testTemplateYaml = `
stage.json {
		expressions = { "app" = "app", "level" = "level" }
}
stage.template {
		source = "app"
		template = "{{ .Value | ToUpper }} doki"
}
stage.template {
		source = "level"
		template = "{{ if eq .Value \"WARN\" }}{{ Replace .Value \"WARN\" \"OK\" -1 }}{{ else }}{{ .Value }}{{ end }}"
}
stage.template {
		source = "nonexistent"
		template = "TEST"
}
stage.labels {
		values = { "app" = "", "level" = "", "type" = "nonexistent" }
}
`

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

func TestPipeline_Template(t *testing.T) {
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testTemplateYaml), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	expectedLbls := model.LabelSet{
		"app":   "LOKI doki",
		"level": "OK",
		"type":  "TEST",
	}
	out := processEntries(pl, newEntry(nil, nil, testTemplateLogLine, time.Now()))[0]
	assert.Equal(t, expectedLbls, out.Labels)
}

func TestPipelineWithMissingKey_Template(t *testing.T) {
	var buf bytes.Buffer
	w := log.NewSyncWriter(&buf)
	logger := log.NewLogfmtLogger(w)
	pl, err := NewPipeline(logger, loadConfig(testTemplateYaml), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}

	_ = processEntries(pl, newEntry(nil, nil, testTemplateLogLineWithMissingKey, time.Now()))

	expectedLog := "level=debug msg=\"extracted template could not be converted to a string\" err=\"can't convert <nil> to string\" type=null"
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

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

func TestTemplateStage_Process(t *testing.T) {
	type testCase struct {
		name              string
		config            TemplateConfig
		extracted         map[string]any
		expectedExtracted map[string]any
	}

	tests := []testCase{
		{
			name: "simple template",
			config: TemplateConfig{
				Source:   "some",
				Template: mustTemplate("{{ .Value }} appended"),
			},
			extracted: map[string]any{
				"some": "value",
			},
			expectedExtracted: map[string]any{
				"some": "value appended",
			},
		},
		{
			name: "add missing",
			config: TemplateConfig{
				Source:   "missing",
				Template: mustTemplate("newval"),
			},
			extracted: map[string]any{
				"notmissing": "value",
			},
			expectedExtracted: map[string]any{
				"notmissing": "value",
				"missing":    "newval",
			},
		},
		{
			name: "template with multiple keys",
			config: TemplateConfig{
				Source:   "message",
				Template: mustTemplate("{{.Value}} in module {{.module}}"),
			},
			extracted: map[string]any{
				"level":   "warn",
				"app":     "loki",
				"message": "warn for app loki",
				"module":  "test",
			},
			expectedExtracted: map[string]any{
				"level":   "warn",
				"app":     "loki",
				"module":  "test",
				"message": "warn for app loki in module test",
			},
		},
		{
			name: "template with multiple keys with missing source",
			config: TemplateConfig{
				Source:   "missing",
				Template: mustTemplate("{{ .level }} for app {{ .app | ToUpper }}"),
			},
			extracted: map[string]any{
				"level": "warn",
				"app":   "loki",
			},
			expectedExtracted: map[string]any{
				"level":   "warn",
				"app":     "loki",
				"missing": "warn for app LOKI",
			},
		},
		{
			name: "template with multiple keys with missing key",
			config: TemplateConfig{
				Source:   "message",
				Template: mustTemplate("{{.Value}} in module {{.module}}"),
			},
			extracted: map[string]any{
				"level":   "warn",
				"app":     "loki",
				"message": "warn for app loki",
			},
			expectedExtracted: map[string]any{
				"level":   "warn",
				"app":     "loki",
				"message": "warn for app loki in module <no value>",
			},
		},
		{
			name: "template with multiple keys with nil value in extracted key",
			config: TemplateConfig{
				Source:   "level",
				Template: mustTemplate("{{ Replace .Value \"Warning\" \"warn\" 1 }}"),
			},
			extracted: map[string]any{
				"level":   "Warning",
				"testval": nil,
			},
			expectedExtracted: map[string]any{
				"level":   "warn",
				"testval": nil,
			},
		},
		{
			name: "ToLower",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ .Value | ToLower }}"),
			},
			extracted: map[string]any{
				"testval": "Value",
			},
			expectedExtracted: map[string]any{
				"testval": "value",
			},
		},
		{
			name: "sprig",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ add 7 3 }}"),
			},
			extracted: map[string]any{
				"testval": "Value",
			},
			expectedExtracted: map[string]any{
				"testval": "10",
			},
		},
		{
			name: "ToLowerParams",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ ToLower .Value }}"),
			},
			extracted: map[string]any{
				"testval": "Value",
			},
			expectedExtracted: map[string]any{
				"testval": "value",
			},
		},
		{
			name: "ToLowerEmptyValue",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ .Value | ToLower }}"),
			},
			extracted:         map[string]any{},
			expectedExtracted: map[string]any{},
		},
		{
			name: "ReplaceAllToLower",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ Replace .Value \" \" \"_\" -1 | ToLower }}"),
			},
			extracted: map[string]any{
				"testval": "Some Silly Value With Lots Of Spaces",
			},
			expectedExtracted: map[string]any{
				"testval": "some_silly_value_with_lots_of_spaces",
			},
		},
		{
			name: "regexReplaceAll",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate(`{{ regexReplaceAll "(Silly)" .Value "${1}foo"  }}`),
			},
			extracted: map[string]any{
				"testval": "Some Silly Value With Lots Of Spaces",
			},
			expectedExtracted: map[string]any{
				"testval": "Some Sillyfoo Value With Lots Of Spaces",
			},
		},
		{
			name: "regexReplaceAllerr",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate(`{{ regexReplaceAll "\\K" .Value "${1}foo"  }}`),
			},
			extracted: map[string]any{
				"testval": "Some Silly Value With Lots Of Spaces",
			},
			expectedExtracted: map[string]any{
				"testval": "Some Silly Value With Lots Of Spaces",
			},
		},
		{
			name: "regexReplaceAllLiteral",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate(`{{ regexReplaceAll "( |Of)" .Value "_"  }}`),
			},
			extracted: map[string]any{
				"testval": "Some Silly Value With Lots Of Spaces",
			},
			expectedExtracted: map[string]any{
				"testval": "Some_Silly_Value_With_Lots___Spaces",
			},
		},
		{
			name: "regexReplaceAllLiteralerr",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate(`{{ regexReplaceAll "\\K" .Value "err"  }}`),
			},
			extracted: map[string]any{
				"testval": "Some Silly Value With Lots Of Spaces",
			},
			expectedExtracted: map[string]any{
				"testval": "Some Silly Value With Lots Of Spaces",
			},
		},
		{
			name: "Trim",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ Trim .Value \"!\" }}"),
			},
			extracted: map[string]any{
				"testval": "!!!!!WOOOOO!!!!!",
			},
			expectedExtracted: map[string]any{
				"testval": "WOOOOO",
			},
		},
		{
			name: "Remove label empty value",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate(""),
			},
			extracted: map[string]any{
				"testval": "WOOOOO",
			},
			expectedExtracted: map[string]any{},
		},
		{
			name: "Don't add label with empty value",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate(""),
			},
			extracted:         map[string]any{},
			expectedExtracted: map[string]any{},
		},
		{
			name: "Sha2Hash",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ Sha2Hash .Value \"salt\" }}"),
			},
			extracted: map[string]any{
				"testval": "this is PII data",
			},
			expectedExtracted: map[string]any{
				"testval": "5526fd6f8ad457279cf8ff06453c6cb61bf479fa826e3b099caa6c846f9376f2",
			},
		},
		{
			name: "Hash",
			config: TemplateConfig{
				Source:   "testval",
				Template: mustTemplate("{{ Hash .Value \"salt\" }}"),
			},
			extracted: map[string]any{
				"testval": "this is PII data",
			},
			expectedExtracted: map[string]any{
				"testval": "0807ea24e992127128b38e4930f7155013786a4999c73a25910318a793847658",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st, err := newTemplateStage(log.NewNopLogger(), tt.config)
			require.NoError(t, err)
			out := processEntries(st, newEntry(tt.extracted, nil, "not important for this test", time.Time{}))[0]
			assert.Equal(t, tt.expectedExtracted, out.Extracted)
		})
	}
}

func BenchmarkTemplateStage(b *testing.B) {
	var entry Entry
	gen := func(n int) map[string]any {
		m := make(map[string]any, n)
		for i := 0; i <= n; i++ {
			v := strconv.FormatInt(int64(i), 10)
			m[v] = v
		}
		return m
	}

	st, err := newTemplateStage(log.NewNopLogger(), TemplateConfig{
		Source:   "1",
		Template: mustTemplate("{{ .Value }}"),
	})
	require.NoError(b, err)
	entry = newEntry(gen(10), nil, "", time.Now())

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		entry = processEntries(st, entry)[0]
	}
}

func mustTemplate(text string) Template {
	t := Template(text)
	err := t.UnmarshalText([]byte(text))
	if err != nil {
		panic(err)
	}
	return t
}
