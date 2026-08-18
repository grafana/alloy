package stages

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/loki/pkg/push"
)

var testLabelsYaml = ` stage.json {
                           expressions = { level = "", app_rename = "app" }
                       }
                       stage.labels { 
                           values = {"level" = "", "app" = "app_rename" }
                       }`

var testLabelsLogLine = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN"
}
`
var testLabelsLogLineWithMissingKey = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"]
}
`

var testLabelsStrucuturedMetadataYaml = `
// Create strucutured metadata
stage.static_labels {
	values = {
	  "foo" = "bar",
	}
}
stage.structured_metadata {
	values = {
	  "baz" = "foo",
	}
}

// Create label from structured metadata
stage.labels {
    source_type = "structured_metadata"
	values      = {
	  "from_structured" = "baz",
	}
}
`

func TestLabelsPipeline_LabelsFromExtracted(t *testing.T) {
	pl, err := NewPipeline(logging.NewSlogNop(), loadConfig(testLabelsYaml), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	expectedLbls := model.LabelSet{
		"level": "WARN",
		"app":   "loki",
	}

	out := processEntries(pl, newEntry(nil, nil, testLabelsLogLine, time.Now()))[0]
	assert.Equal(t, expectedLbls, out.Labels)
}

func TestLabelsPipeline_LabelsFromStructuredMetadata(t *testing.T) {
	pl, err := NewPipeline(logging.NewSlogNop(), loadConfig(testLabelsStrucuturedMetadataYaml), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	expectedLbls := model.LabelSet{
		"from_structured": "bar",
	}

	out := processEntries(pl, newEntry(nil, nil, "", time.Now()))[0]
	assert.Equal(t, expectedLbls, out.Labels)
}

func TestLabelsPipelineWithMissingKey_Labels(t *testing.T) {
	var buf bytes.Buffer
	alloyLogger, err := logging.New(&buf, logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
	assert.NoError(t, err)
	pl, err := NewPipeline(alloyLogger.Slog(), loadConfig(testLabelsYaml), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	_ = processEntries(pl, newEntry(nil, nil, testLabelsLogLineWithMissingKey, time.Now()))

	expectedLog := "level=debug msg=\"failed to convert extracted label value to string\" stage=labels err=\"can't convert <nil> to string\" type=<nil>"
	if !strings.Contains(buf.String(), expectedLog) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

func TestLabelsStage(t *testing.T) {
	sourceName := "diff_source"

	type testCase struct {
		name     string
		cfg      LabelsConfig
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "extract success extracted",
			cfg: LabelsConfig{Values: map[string]*string{
				"testLabel": nil,
			}},
			entries: []Entry{
				newTestEntry(map[string]any{"testLabel": "testValue"}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"testLabel": "testValue"}, model.LabelSet{
					"testLabel": "testValue",
				}, push.Entry{}),
			},
		},
		{
			name: "extract success structured metadata",
			cfg: LabelsConfig{
				SourceType: SourceTypeStructuredMetadata,
				Values: map[string]*string{
					"testLabel": ptr("testStrucuturedMetadata"),
				},
			},
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: "testStrucuturedMetadata", Value: "testValue"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{
					"testLabel": "testValue",
				}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: "testStrucuturedMetadata", Value: "testValue"},
					},
				}),
			},
		},
		{
			name: "different source name extracted",
			cfg: LabelsConfig{Values: map[string]*string{
				"testLabel": &sourceName,
			}},
			entries: []Entry{
				newTestEntry(map[string]any{sourceName: "testValue"}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{sourceName: "testValue"}, model.LabelSet{
					"testLabel": "testValue",
				}, push.Entry{}),
			},
		},
		{
			name: "different source name structured metadata",
			cfg: LabelsConfig{
				SourceType: SourceTypeStructuredMetadata,
				Values: map[string]*string{
					"testLabel": &sourceName,
				},
			},
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: sourceName, Value: "testValue"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{
					"testLabel": "testValue",
				}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: sourceName, Value: "testValue"},
					},
				}),
			},
		},
		{
			name: "empty extracted data",
			cfg: LabelsConfig{Values: map[string]*string{
				"testLabel": &sourceName,
			}},
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{}),
			},
		},
		{
			name: "empty structured metadata",
			cfg: LabelsConfig{
				SourceType: SourceTypeStructuredMetadata,
				Values: map[string]*string{
					"testLabel": &sourceName,
				},
			},
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{LabelsConfig: &tt.cfg}}, tt.entries, tt.expected, "")
		})
	}
}

func TestValidateLabelsConfig(t *testing.T) {
	var (
		lv1 = "lv1"
		lv3 = ""
	)

	tests := map[string]struct {
		config       LabelsConfig
		err          error
		expectedCfgs map[string]string
	}{
		"missing config": {
			config:       LabelsConfig{},
			err:          errors.New(errEmptyLabelStageConfig),
			expectedCfgs: nil,
		},
		"invalid label name": {
			config: LabelsConfig{
				Values: map[string]*string{"\xfd": nil},
			},
			err:          fmt.Errorf(errInvalidLabelName, "\xfd"),
			expectedCfgs: nil,
		},
		"invalid source type": {
			config: LabelsConfig{
				Values:     map[string]*string{"l1": ptr("")},
				SourceType: "invalid_source_type",
			},
			err:          fmt.Errorf("invalid labels source_type: %s. Can only be 'extracted' or 'structured_metadata'", "invalid_source_type"),
			expectedCfgs: nil,
		},
		"label value is set from name for extracted": {
			config: LabelsConfig{
				SourceType: SourceTypeExtractedMap,
				Values: map[string]*string{
					"l1": &lv1,
					"l2": nil,
					"l3": &lv3,
				}},
			err: nil,
			expectedCfgs: map[string]string{
				"l1": lv1,
				"l2": "l2",
				"l3": "l3",
			},
		},
		"label value is set from name for structured_metadata": {
			config: LabelsConfig{
				SourceType: SourceTypeStructuredMetadata,
				Values: map[string]*string{
					"l1": &lv1,
					"l2": nil,
					"l3": &lv3,
				}},
			err: nil,
			expectedCfgs: map[string]string{
				"l1": lv1,
				"l2": "l2",
				"l3": "l3",
			},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			actual, err := validateLabelsConfig(&test.config)
			if (err != nil) != (test.err != nil) {
				t.Errorf("validateLabelsConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if (err != nil) && (err.Error() != test.err.Error()) {
				t.Errorf("validateLabelsConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if test.expectedCfgs != nil {
				assert.Equal(t, test.expectedCfgs, actual)
			}
		})
	}
}
