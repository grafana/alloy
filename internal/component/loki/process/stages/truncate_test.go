package stages

import (
	"errors"
	"testing"
	"time"

	dskit "github.com/grafana/dskit/server"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
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
					Limit:  1000,
					Suffix: "...",
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
					Limit: 10,
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
					Limit:  10,
					Suffix: "...",
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
					SourceType: TruncateSourceLabel,
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
					SourceType: TruncateSourceLabel,
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
					SourceType: TruncateSourceStructuredMetadata,
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
					SourceType: TruncateSourceStructuredMetadata,
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
					Limit: 10,
				},
				{
					Limit:      15,
					SourceType: TruncateSourceLabel,
					Suffix:     "[truncated]",
					Sources:    []string{"app"},
				},
				{
					Limit:      15,
					SourceType: TruncateSourceStructuredMetadata,
					Suffix:     "<trunc>",
				},
				{
					Limit:      8,
					SourceType: TruncateSourceExtractedMap,
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
			err := validateTruncateConfig(cfg)
			if err != nil {
				t.Error(err)
			}
			logger := util.TestAlloyLogger(t)
			registry := prometheus.NewRegistry()
			m, err := newTruncateStage(logger, *cfg, registry)
			require.NoError(t, err)
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

func Test_ValidateTruncateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *TruncateConfig
		wantErr error
	}{
		{
			name: "Error no rules",
			config: &TruncateConfig{
				Rules: []*RuleConfig{},
			},
			wantErr: errors.New(errAtLeastOneRule),
		},
		{
			name: "Error limit must be positive",
			config: &TruncateConfig{
				Rules: []*RuleConfig{{
					Limit: 0,
				}},
			},
			wantErr: errors.New(errLimitMustBeGreaterThanZero),
		},
		{
			name: "Error sources cannot be set for line",
			config: &TruncateConfig{
				Rules: []*RuleConfig{{
					Limit:   10,
					Sources: []string{"app"},
				}},
			},
			wantErr: errors.New(errSourcesForLine),
		},
		{
			name: "No error for intrinsic line limit",
			config: &TruncateConfig{
				Rules: []*RuleConfig{{
					Limit:  10,
					Suffix: "...",
				}},
			},
			wantErr: nil,
		},
		{
			name: "No error for label limit",
			config: &TruncateConfig{
				Rules: []*RuleConfig{{
					Limit:      10,
					SourceType: TruncateSourceLabel,
					Suffix:     "...",
				}},
			},
			wantErr: nil,
		},
		{
			name: "No error for structured_metadata limit",
			config: &TruncateConfig{
				Rules: []*RuleConfig{{
					Limit:      10,
					SourceType: TruncateSourceStructuredMetadata,
					Suffix:     "...",
				}},
			},
			wantErr: nil,
		},
		{
			name: "No error for specific label limit",
			config: &TruncateConfig{
				Rules: []*RuleConfig{
					{
						Limit:      10,
						SourceType: TruncateSourceLabel,
						Sources:    []string{"app"},
						Suffix:     "...",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "Suffix too long",
			config: &TruncateConfig{
				Rules: []*RuleConfig{
					{
						Limit:  10,
						Suffix: "12345678901",
					},
				},
			},
			wantErr: errors.New(`suffix length cannot be greater than or equal to limit`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTruncateConfig(tt.config)
			if tt.wantErr != nil {
				require.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func ptr[T any](s T) *T {
	return &s
}
