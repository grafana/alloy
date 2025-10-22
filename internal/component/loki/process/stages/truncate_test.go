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
		config                     *TruncateConfig
		labels                     map[string]string
		structured_metadata        push.LabelsAdapter
		t                          time.Time
		entry                      string
		expectedEntry              *string
		expectedLabels             *map[string]string
		expectedStructuredMetadata *push.LabelsAdapter
		incrementedCount           *map[string]float64
	}{
		{
			name: "passthrough when under limits",
			config: &TruncateConfig{
				LineLimit: 1000,
				Suffix:    "...",
			},
			labels:              map[string]string{},
			structured_metadata: push.LabelsAdapter{},
			entry:               "12345678901",
		},
		{
			name: "Longer line should truncate",
			config: &TruncateConfig{
				LineLimit: 10,
			},
			labels:              map[string]string{},
			structured_metadata: push.LabelsAdapter{},
			entry:               "123456789012",
			expectedEntry:       ptr("1234567890"),
			incrementedCount:    ptr(map[string]float64{"line": 1}),
		}, {
			name: "Longer line should truncate with suffix",
			config: &TruncateConfig{
				LineLimit: 10,
				Suffix:    "...",
			},
			labels:              map[string]string{},
			structured_metadata: push.LabelsAdapter{},
			entry:               "12345678901",
			expectedEntry:       ptr("1234567..."),
			incrementedCount:    ptr(map[string]float64{"line": 1}),
		},
		{
			name: "Longer labels should truncate",
			config: &TruncateConfig{
				LabelLimit: 15,
				Suffix:     "[truncated]",
			},
			labels:              map[string]string{"app": "my-very-long-app-name", "version": "1.0.0-experimental", "env": "prod"},
			expectedLabels:      ptr(map[string]string{"app": "my-v[truncated]", "version": "1.0.[truncated]", "env": "prod"}),
			structured_metadata: push.LabelsAdapter{},
			entry:               "12345678901",
			incrementedCount:    ptr(map[string]float64{"label": 2}),
		},
		{
			name: "Longer structured_metadata should truncate",
			config: &TruncateConfig{
				StructuredMetadataLimit: 15,
				Suffix:                  "<trunc>",
			},
			labels:              map[string]string{"app": "my-very-long-app-name", "env": "prod"},
			structured_metadata: push.LabelsAdapter{push.LabelAdapter{Name: "meta1", Value: "my-very-long-metadata-value"}, push.LabelAdapter{Name: "meta2", Value: "short"}},
			expectedStructuredMetadata: ptr(push.LabelsAdapter{
				push.LabelAdapter{Name: "meta1", Value: "my-very-<trunc>"},
				push.LabelAdapter{Name: "meta2", Value: "short"},
			}),
			entry:            "12345678901",
			incrementedCount: ptr(map[string]float64{"structured_metadata": 1}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTruncateConfig(tt.config)
			if err != nil {
				t.Error(err)
			}
			logger := util.TestAlloyLogger(t)
			registry := prometheus.NewRegistry()
			m, err := newTruncateStage(logger, *tt.config, registry)
			require.NoError(t, err)
			entry := newEntry(map[string]interface{}{}, toLabelSet(tt.labels), tt.entry, tt.t)
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
			name:    "ErrEmpty",
			config:  &TruncateConfig{},
			wantErr: errors.New(errTruncateStageEmptyConfig),
		},
		{
			name: "No error for line limit only",
			config: &TruncateConfig{
				LineLimit: 10,
				Suffix:    "...",
			},
			wantErr: nil,
		},
		{
			name: "No error for label limit only",
			config: &TruncateConfig{
				LabelLimit: 10,
				Suffix:     "...",
			},
			wantErr: nil,
		},
		{
			name: "No error for structured_metadata limit only",
			config: &TruncateConfig{
				StructuredMetadataLimit: 10,
				Suffix:                  "...",
			},
			wantErr: nil,
		},
		{
			name: "No error for all limits set",
			config: &TruncateConfig{
				LineLimit:               10,
				LabelLimit:              5,
				StructuredMetadataLimit: 10,
				Suffix:                  "...",
			},
			wantErr: nil,
		},
		{
			name: "Suffix too long",
			config: &TruncateConfig{
				LineLimit:               10,
				LabelLimit:              50,
				StructuredMetadataLimit: 10,
				Suffix:                  "12345678901",
			},
			wantErr: errors.New(`suffix length cannot be greater than or equal to line_limit
suffix length cannot be greater than or equal to structured_metadata_limit`),
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
