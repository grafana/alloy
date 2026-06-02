package stages

import (
	"testing"
	"time"

	dskit "github.com/grafana/dskit/server"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func Test_TruncateStage_Process(t *testing.T) {
	// Enable debug logging
	cfg := &dskit.Config{}
	require.Nil(t, cfg.LogLevel.Set("debug"))
	Debug = true

	tests := []struct {
		name                       string
		config                     []*RuleConfig
		t                          time.Time
		entry                      string
		expectedEntry              *string
		labels                     map[string]string
		expectedLabels             *map[string]string
		structured_metadata        push.LabelsAdapter
		expectedStructuredMetadata *push.LabelsAdapter
		extracted                  map[string]any
		expectedExtracted          *map[string]any
		incrementedCount           *map[string]float64
	}{
		{
			name: "passthrough when under limits",
			config: []*RuleConfig{
				{
					Limit:      1000,
					Suffix:     "...",
					SourceType: SourceTypeLine,
				},
			},
			labels:              map[string]string{},
			structured_metadata: push.LabelsAdapter{},
			extracted:           map[string]any{},
			entry:               "12345678901",
		},
		{
			name: "Longer line should truncate",
			config: []*RuleConfig{
				{
					Limit:      10,
					SourceType: SourceTypeLine,
				},
			},
			labels:              map[string]string{},
			structured_metadata: push.LabelsAdapter{},
			entry:               "123456789012",
			expectedEntry:       ptr("1234567890"),
			expectedExtracted:   ptr(map[string]any{"truncated": "line"}),
			incrementedCount:    ptr(map[string]float64{"line": 1}),
		}, {
			name: "Longer line should truncate with suffix",
			config: []*RuleConfig{
				{
					Limit:      10,
					Suffix:     "...",
					SourceType: SourceTypeLine,
				},
			},
			labels:              map[string]string{},
			structured_metadata: push.LabelsAdapter{},
			entry:               "12345678901",
			expectedEntry:       ptr("1234567..."),
			expectedExtracted:   ptr(map[string]any{"truncated": "line"}),
			incrementedCount:    ptr(map[string]float64{"line": 1}),
		},
		{
			name: "Longer labels should truncate",
			config: []*RuleConfig{
				{
					Limit:      15,
					SourceType: SourceTypeLabel,
					Suffix:     "[truncated]",
				},
			},
			labels:              map[string]string{"app": "my-very-long-app-name", "version": "1.0.0-experimental", "env": "prod"},
			expectedLabels:      ptr(map[string]string{"app": "my-v[truncated]", "version": "1.0.[truncated]", "env": "prod"}),
			structured_metadata: push.LabelsAdapter{},
			entry:               "12345678901",
			expectedExtracted:   ptr(map[string]any{"truncated": "label"}),
			incrementedCount:    ptr(map[string]float64{"label": 2}),
		},
		{
			name: "Only specified sources should truncate in labels",
			config: []*RuleConfig{
				{
					Limit:      15,
					SourceType: SourceTypeLabel,
					Suffix:     "[truncated]",
					Sources:    []string{"app"},
				},
			},
			labels:              map[string]string{"app": "my-very-long-app-name", "version": "1.0.0-experimental", "env": "prod"},
			expectedLabels:      ptr(map[string]string{"app": "my-v[truncated]", "version": "1.0.0-experimental", "env": "prod"}),
			structured_metadata: push.LabelsAdapter{},
			entry:               "12345678901",
			expectedExtracted:   ptr(map[string]any{"truncated": "label"}),
			incrementedCount:    ptr(map[string]float64{"label": 1}),
		},
		{
			name: "Longer structured_metadata should truncate",
			config: []*RuleConfig{
				{
					Limit:      15,
					SourceType: SourceTypeStructuredMetadata,
					Suffix:     "<trunc>",
				},
			},
			labels:              map[string]string{"app": "my-very-long-app-name", "env": "prod"},
			structured_metadata: push.LabelsAdapter{push.LabelAdapter{Name: "meta1", Value: "my-very-long-metadata-value"}, push.LabelAdapter{Name: "meta2", Value: "short"}},
			expectedStructuredMetadata: ptr(push.LabelsAdapter{
				push.LabelAdapter{Name: "meta1", Value: "my-very-<trunc>"},
				push.LabelAdapter{Name: "meta2", Value: "short"},
			}),
			entry:             "12345678901",
			expectedExtracted: ptr(map[string]any{"truncated": "structured_metadata"}),
			incrementedCount:  ptr(map[string]float64{"structured_metadata": 1}),
		},
		{
			name: "Only specified structured_metadata should truncate",
			config: []*RuleConfig{
				{
					Limit:      15,
					SourceType: SourceTypeStructuredMetadata,
					Suffix:     "<trunc>",
					Sources:    []string{"meta1"},
				},
			},
			labels:              map[string]string{"app": "my-very-long-app-name", "env": "prod"},
			structured_metadata: push.LabelsAdapter{push.LabelAdapter{Name: "meta1", Value: "my-very-long-metadata-value"}, push.LabelAdapter{Name: "meta2", Value: "another long value"}},
			expectedStructuredMetadata: ptr(push.LabelsAdapter{
				push.LabelAdapter{Name: "meta1", Value: "my-very-<trunc>"},
				push.LabelAdapter{Name: "meta2", Value: "another long value"},
			}),
			entry:             "12345678901",
			expectedExtracted: ptr(map[string]any{"truncated": "structured_metadata"}),
			incrementedCount:  ptr(map[string]float64{"structured_metadata": 1}),
		},
		{
			name: "Multiple rules applied together",
			config: []*RuleConfig{
				{
					Limit:      10,
					SourceType: SourceTypeLine,
				},
				{
					Limit:      15,
					SourceType: SourceTypeLabel,
					Suffix:     "[truncated]",
					Sources:    []string{"app"},
				},
				{
					Limit:      15,
					SourceType: SourceTypeStructuredMetadata,
					Suffix:     "<trunc>",
				},
				{
					Limit:      8,
					SourceType: SourceTypeExtractedMap,
					Sources:    []string{"field2"},
				},
			},
			entry:               "12345678901234",
			expectedEntry:       ptr("1234567890"),
			labels:              map[string]string{"app": "my-very-long-app-name", "version": "1.0.0-experimental", "env": "prod"},
			expectedLabels:      ptr(map[string]string{"app": "my-v[truncated]", "version": "1.0.0-experimental", "env": "prod"}),
			structured_metadata: push.LabelsAdapter{push.LabelAdapter{Name: "meta1", Value: "my-very-long-metadata-value"}, push.LabelAdapter{Name: "meta2", Value: "another long value"}},
			expectedStructuredMetadata: ptr(push.LabelsAdapter{
				push.LabelAdapter{Name: "meta1", Value: "my-very-<trunc>"},
				push.LabelAdapter{Name: "meta2", Value: "another <trunc>"},
			}),
			extracted: map[string]any{"field1": "this is kind of long", "field2": "this-is-a-very-long-field-value"},
			expectedExtracted: &map[string]any{
				"field1": "this is kind of long", "field2": "this-is-", "truncated": "extracted,label,line,structured_metadata",
			},
			incrementedCount: ptr(map[string]float64{"structured_metadata": 2, "line": 1, "label": 1, "extracted": 1}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TruncateConfig{Rules: tt.config}

			logger := util.TestAlloyLogger(t)
			registry := prometheus.NewRegistry()
			m := newTruncateStage(logger, *cfg, registry)
			entry := newEntry(map[string]any{}, toLabelSet(tt.labels), tt.entry, tt.t)
			if tt.extracted != nil {
				entry.Extracted = tt.extracted
			}
			entry.StructuredMetadata = tt.structured_metadata
			out := processEntries(m, entry)
			if tt.expectedEntry != nil {
				require.Equal(t, *tt.expectedEntry, out[0].Line)

				require.Contains(t, out[0].Extracted, "truncated")
				require.Contains(t, out[0].Extracted["truncated"], "line")
			} else {
				require.Equal(t, tt.entry, out[0].Line)
			}
			if tt.expectedLabels != nil {
				assertLabels(t, *tt.expectedLabels, out[0].Labels)

				require.Contains(t, out[0].Extracted, "truncated")
				require.Contains(t, out[0].Extracted["truncated"], "label")
			} else {
				assertLabels(t, tt.labels, out[0].Labels)
			}
			if tt.expectedStructuredMetadata != nil {
				require.Equal(t, *tt.expectedStructuredMetadata, out[0].StructuredMetadata)

				require.Contains(t, out[0].Extracted, "truncated")
				require.Contains(t, out[0].Extracted["truncated"], "structured_metadata")
			} else {
				require.Equal(t, tt.structured_metadata, out[0].StructuredMetadata)
			}

			if tt.expectedExtracted != nil {
				require.Equal(t, *tt.expectedExtracted, out[0].Extracted)
			} else {
				require.Equal(t, tt.extracted, out[0].Extracted)
			}

			if tt.incrementedCount != nil {
				mfs, _ := registry.Gather()
				require.Len(t, mfs, 1)
				require.Equal(t, "loki_process_truncated_fields_total", *mfs[0].Name)
				// Check all expected counts are present
				for k, v := range *tt.incrementedCount {
					for _, mf := range mfs[0].Metric {
						if *mf.Label[0].Value == k && mf.Label[0].GetName() == "field" {
							require.Equal(t, v, mf.GetCounter().GetValue(), "expected truncated count for type %s to be %d", k, v)
						}
					}
				}
			}
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
