package stages

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestLabelsStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "labels from extracted",
			config: `
			stage.json {
				expressions = { level = "", app_rename = "app" }
			}
			stage.labels {
				values = { "level" = "", "app" = "app_rename" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"time":"2012-11-01T22:08:41+00:00", "app":"loki", "component": ["parser","type"], "level" : "WARN"}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"level":      "WARN",
					"app_rename": "loki",
				}, model.LabelSet{
					"level": "WARN",
					"app":   "loki",
				}, `{"time":"2012-11-01T22:08:41+00:00", "app":"loki", "component": ["parser","type"], "level" : "WARN"}`, now),
			},
		},
		{
			name: "missing key skips label conversion",
			config: `
			stage.json {
				expressions = { level = "", app_rename = "app" }
			}
			stage.labels {
				values = { "level" = "", "app" = "app_rename" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"time":"2012-11-01T22:08:41+00:00", "app":"loki", "component": ["parser","type"]}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"app_rename": "loki",
					"level":      nil,
				}, model.LabelSet{
					"app": "loki",
				}, `{"time":"2012-11-01T22:08:41+00:00", "app":"loki", "component": ["parser","type"]}`, now),
			},
		},
		{
			name: "labels from structured metadata",
			config: `
			stage.static_labels {
				values = { "foo" = "bar" }
			}
			stage.structured_metadata {
				values = { "baz" = "foo" }
			}
			stage.labels {
				source_type = "structured_metadata"
				values = { "from_structured" = "baz" }
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{
					"from_structured": "bar",
				}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: "baz", Value: "bar"},
					},
				}),
			},
		},
		{
			name: "extract success extracted",
			config: `
			stage.labels {
				values = { "testLabel" = "" }
			}
			`,
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
			config: `
			stage.labels {
				source_type = "structured_metadata"
				values = { "testLabel" = "testStrucuturedMetadata" }
			}
			`,
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
			config: `
			stage.labels {
				values = { "testLabel" = "diff_source" }
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{"diff_source": "testValue"}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"diff_source": "testValue"}, model.LabelSet{
					"testLabel": "testValue",
				}, push.Entry{}),
			},
		},
		{
			name: "different source name structured metadata",
			config: `
			stage.labels {
				source_type = "structured_metadata"
				values = { "testLabel" = "diff_source" }
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: "diff_source", Value: "testValue"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{
					"testLabel": "testValue",
				}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: "diff_source", Value: "testValue"},
					},
				}),
			},
		},
		{
			name: "empty extracted data",
			config: `
			stage.labels {
				values = { "testLabel" = "diff_source" }
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{}),
			},
		},
		{
			name: "empty structured metadata",
			config: `
			stage.labels {
				source_type = "structured_metadata"
				values = { "testLabel" = "diff_source" }
			}
			`,
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

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected)
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
