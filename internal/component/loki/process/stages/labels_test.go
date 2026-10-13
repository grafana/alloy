package stages

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
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
			name: "default source names extracted",
			config: `
			stage.labels {
				values = { "l1" = "source", "l2" = null, "l3" = "" }
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{"source": "v1", "l2": "v2", "l3": "v3"}, model.LabelSet{}, push.Entry{}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"source": "v1", "l2": "v2", "l3": "v3"}, model.LabelSet{
					"l1": "v1",
					"l2": "v2",
					"l3": "v3",
				}, push.Entry{}),
			},
		},
		{
			name: "default source names structured metadata",
			config: `
			stage.labels {
				source_type = "structured_metadata"
				values = { "l1" = "source", "l2" = null, "l3" = "" }
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: "source", Value: "v1"},
						{Name: "l2", Value: "v2"},
						{Name: "l3", Value: "v3"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{
					"l1": "v1",
					"l2": "v2",
					"l3": "v3",
				}, push.Entry{
					StructuredMetadata: push.LabelsAdapter{
						{Name: "source", Value: "v1"},
						{Name: "l2", Value: "v2"},
						{Name: "l3", Value: "v3"},
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
	tests := []struct {
		name      string
		config    string
		expectErr bool
	}{
		{
			name:   "valid",
			config: `values = { "testLabel" = "source" }`,
		},
		{
			name:   "null value uses label name",
			config: `values = { "testLabel" = null }`,
		},
		{
			name:   "empty value uses label name",
			config: `values = { "testLabel" = "" }`,
		},
		{
			name:      "missing config",
			config:    ``,
			expectErr: true,
		},
		{
			name:      "null config",
			config:    `values = null`,
			expectErr: true,
		},
		{
			name:      "invalid label name",
			config:    "values = { \"\xfd\" = \"source\" }",
			expectErr: true,
		},
		{
			name: "extracted source type",
			config: `
			values = { "testLabel" = "source" }
			source_type = "extracted"
			`,
		},
		{
			name: "structured metadata source type",
			config: `
			values = { "testLabel" = "source" }
			source_type = "structured_metadata"
			`,
		},
		{
			name: "invalid source type",
			config: `
			values = { "testLabel" = "source" }
			source_type = "invalid_source_type"
			`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg LabelsConfig
			err := syntax.Unmarshal([]byte(tt.config), &cfg)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
