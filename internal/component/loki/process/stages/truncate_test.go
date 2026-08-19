package stages

import (
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

const truncateMetricsHeader = `
# HELP loki_process_truncated_fields_total A count of all log lines, labels, extracted values, or structured_metadata truncated as a result of a pipeline stage
# TYPE loki_process_truncated_fields_total counter
`

func TestTruncateStage(t *testing.T) {
	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
		metrics  string
	}

	tests := []testCase{
		{
			name: "passthrough when under limits",
			config: `
			stage.truncate {
				rule {
					limit       = "1000B"
					suffix      = "..."
					source_type = "line"
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{Line: "12345678901", StructuredMetadata: push.LabelsAdapter{}}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{Line: "12345678901", StructuredMetadata: push.LabelsAdapter{}}),
			},
		},
		{
			name: "Longer line should truncate",
			config: `
			stage.truncate {
				rule {
					limit       = "10B"
					source_type = "line"
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{Line: "123456789012", StructuredMetadata: push.LabelsAdapter{}}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"truncated": "line"}, model.LabelSet{}, push.Entry{Line: "1234567890", StructuredMetadata: push.LabelsAdapter{}}),
			},
			metrics: "loki_process_truncated_fields_total{field=\"line\"} 1\n",
		},
		{
			name: "Longer line should truncate with suffix",
			config: `
			stage.truncate {
				rule {
					limit       = "10B"
					suffix      = "..."
					source_type = "line"
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{Line: "12345678901", StructuredMetadata: push.LabelsAdapter{}}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"truncated": "line"}, model.LabelSet{}, push.Entry{Line: "1234567...", StructuredMetadata: push.LabelsAdapter{}}),
			},
			metrics: "loki_process_truncated_fields_total{field=\"line\"} 1\n",
		},
		{
			name: "Longer labels should truncate",
			config: `
			stage.truncate {
				rule {
					limit       = "15B"
					source_type = "label"
					suffix      = "[truncated]"
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"app": "my-very-long-app-name", "version": "1.0.0-experimental", "env": "prod"}, push.Entry{Line: "12345678901", StructuredMetadata: push.LabelsAdapter{}}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"app":       "my-very-long-app-name",
					"version":   "1.0.0-experimental",
					"env":       "prod",
					"truncated": "label",
				}, model.LabelSet{"app": "my-v[truncated]", "version": "1.0.[truncated]", "env": "prod"}, push.Entry{Line: "12345678901", StructuredMetadata: push.LabelsAdapter{}}),
			},
			metrics: "loki_process_truncated_fields_total{field=\"label\"} 2\n",
		},
		{
			name: "Only specified sources should truncate in labels",
			config: `
			stage.truncate {
				rule {
					limit       = "15B"
					source_type = "label"
					suffix      = "[truncated]"
					sources     = ["app"]
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"app": "my-very-long-app-name", "version": "1.0.0-experimental", "env": "prod"}, push.Entry{Line: "12345678901", StructuredMetadata: push.LabelsAdapter{}}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"app":       "my-very-long-app-name",
					"version":   "1.0.0-experimental",
					"env":       "prod",
					"truncated": "label",
				}, model.LabelSet{"app": "my-v[truncated]", "version": "1.0.0-experimental", "env": "prod"}, push.Entry{Line: "12345678901", StructuredMetadata: push.LabelsAdapter{}}),
			},
			metrics: "loki_process_truncated_fields_total{field=\"label\"} 1\n",
		},
		{
			name: "Longer structured_metadata should truncate",
			config: `
			stage.truncate {
				rule {
					limit       = "15B"
					source_type = "structured_metadata"
					suffix      = "<trunc>"
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"app": "my-very-long-app-name", "env": "prod"}, push.Entry{
					Line: "12345678901",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "meta1", Value: "my-very-long-metadata-value"},
						{Name: "meta2", Value: "short"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"app":       "my-very-long-app-name",
					"env":       "prod",
					"truncated": "structured_metadata",
				}, model.LabelSet{"app": "my-very-long-app-name", "env": "prod"}, push.Entry{
					Line: "12345678901",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "meta1", Value: "my-very-<trunc>"},
						{Name: "meta2", Value: "short"},
					},
				}),
			},
			metrics: "loki_process_truncated_fields_total{field=\"structured_metadata\"} 1\n",
		},
		{
			name: "Only specified structured_metadata should truncate",
			config: `
			stage.truncate {
				rule {
					limit       = "15B"
					source_type = "structured_metadata"
					suffix      = "<trunc>"
					sources     = ["meta1"]
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"app": "my-very-long-app-name", "env": "prod"}, push.Entry{
					Line: "12345678901",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "meta1", Value: "my-very-long-metadata-value"},
						{Name: "meta2", Value: "another long value"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"app":       "my-very-long-app-name",
					"env":       "prod",
					"truncated": "structured_metadata",
				}, model.LabelSet{"app": "my-very-long-app-name", "env": "prod"}, push.Entry{
					Line: "12345678901",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "meta1", Value: "my-very-<trunc>"},
						{Name: "meta2", Value: "another long value"},
					},
				}),
			},
			metrics: "loki_process_truncated_fields_total{field=\"structured_metadata\"} 1\n",
		},
		{
			name: "Multiple rules applied together",
			config: `
			stage.truncate {
				rule {
					limit       = "10B"
					source_type = "line"
				}
				rule {
					limit       = "15B"
					source_type = "label"
					suffix      = "[truncated]"
					sources     = ["app"]
				}
				rule {
					limit       = "15B"
					source_type = "structured_metadata"
					suffix      = "<trunc>"
				}
				rule {
					limit       = "8B"
					source_type = "extracted"
					sources     = ["field2"]
				}
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{
					"field1": "this is kind of long",
					"field2": "this-is-a-very-long-field-value",
				}, model.LabelSet{"app": "my-very-long-app-name", "version": "1.0.0-experimental", "env": "prod"}, push.Entry{
					Line: "12345678901234",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "meta1", Value: "my-very-long-metadata-value"},
						{Name: "meta2", Value: "another long value"},
					},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"field1":    "this is kind of long",
					"field2":    "this-is-",
					"app":       "my-very-long-app-name",
					"version":   "1.0.0-experimental",
					"env":       "prod",
					"truncated": "extracted,label,line,structured_metadata",
				}, model.LabelSet{"app": "my-v[truncated]", "version": "1.0.0-experimental", "env": "prod"}, push.Entry{
					Line: "1234567890",
					StructuredMetadata: push.LabelsAdapter{
						{Name: "meta1", Value: "my-very-<trunc>"},
						{Name: "meta2", Value: "another <trunc>"},
					},
				}),
			},
			metrics: `
loki_process_truncated_fields_total{field="extracted"} 1
loki_process_truncated_fields_total{field="label"} 1
loki_process_truncated_fields_total{field="line"} 1
loki_process_truncated_fields_total{field="structured_metadata"} 2
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expectedMetrics := tt.metrics
			if expectedMetrics != "" {
				expectedMetrics = truncateMetricsHeader + expectedMetrics
			}

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected, expectedMetrics)
		})
	}
}

func TestTruncateStage_UnmarshalAlloy(t *testing.T) {
	type testCase struct {
		name    string
		config  string
		wantErr bool
	}

	tests := []testCase{
		{
			name:    "empty block",
			config:  ``,
			wantErr: true,
		},
		{
			name: "empty rule",
			config: `
				rule {}
			`,
			wantErr: true,
		},
		{
			name: "unknown source_type",
			config: `
				rule {
					limit = "1b"
					source_type = "test"
				}
			`,
			wantErr: true,
		},
		{
			name: "all attributes",
			config: `
				rule {
					limit = "1MiB"
					source_type = "extracted"
					sources = ["app", "app2"]
					suffix = "..."
				}
			`,
			wantErr: false,
		},
		{
			name: "multiple rules",
			config: `
				rule {
					limit = "1MiB"
					source_type = "line"
					suffix = "..."
				}

				rule {
					limit = "1MiB"
					source_type = "label"
					sources = ["app", "app2"]
					suffix = "..."
				}

				rule {
					limit = "1MiB"
					source_type = "extracted"
					sources = ["app", "app2"]
					suffix = "..."
				}

				rule {
					limit = "1MiB"
					source_type = "structured_metadata"
					sources = ["app", "app2"]
					suffix = "..."
				}
			`,
			wantErr: false,
		},
		{
			name: "limit must be greater than zero",
			config: `
				rule {
					limit = "0B"
				}
			`,
			wantErr: true,
		},
		{
			name: "sources cannot be set when source_type is line",
			config: `
				rule {
					limit = "10B"
					sources = ["app"]
				}
			`,
			wantErr: true,
		},
		{
			name: "suffix length greater than or equal to limit",
			config: `
				rule {
					limit = "10B"
					suffix = "12345678901"
				}
			`,
			wantErr: true,
		},
		{
			name: "intrinsic line limit",
			config: `
				rule {
					limit = "10B"
					suffix = "..."
				}
			`,
			wantErr: false,
		},
		{
			name: "label limit",
			config: `
				rule {
					limit = "10B"
					source_type = "label"
					suffix = "..."
				}
			`,
			wantErr: false,
		},
		{
			name: "structured_metadata limit",
			config: `
				rule {
					limit = "10B"
					source_type = "structured_metadata"
					suffix = "..."
				}
			`,
			wantErr: false,
		},
		{
			name: "specific label limit",
			config: `
				rule {
					limit = "10B"
					source_type = "label"
					sources = ["app"]
					suffix = "..."
				}
			`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg TruncateConfig
			err := syntax.Unmarshal([]byte(tt.config), &cfg)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func ptr[T any](s T) *T {
	return &s
}
