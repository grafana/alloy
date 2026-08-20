package stages

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestStructuredMetadataDropStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "expected structured_metadata_drop to remove one entry",
			config: `
			stage.static_labels {
				values = {"foo" = "bar"}
			}

			stage.structured_metadata {
				values = {"foo" = ""}
			}

			stage.structured_metadata_drop {
				values = ["foo"]
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					StructuredMetadata: push.LabelsAdapter{},
				}),
			},
		},
		{
			name: "expected structured_metadata_drop to remove two entries",
			config: `
			stage.static_labels {
				values = {
				  "foo" = "bar",
				  "bar" = "baz",
				}
			}

			stage.structured_metadata {
				values = {
				  "foo" = "",
				  "bar" = "",
				}
			}

			stage.structured_metadata_drop {
				values = ["foo", "bar"]
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					StructuredMetadata: push.LabelsAdapter{},
				}),
			},
		},
		{
			name: "expected structured_metadata_drop to remove non existing entry",
			config: `
			stage.static_labels {
				values = {
				  "foo" = "bar",
				}
			}

			stage.structured_metadata {
				values = {
				  "foo" = "",
				}
			}

			stage.structured_metadata_drop {
				values = ["baz"]
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					StructuredMetadata: push.LabelsAdapter{{Name: "foo", Value: "bar"}},
				}),
			},
		},
		{
			name: "expected structured_metadata_drop to keep other entries",
			config: `
			stage.static_labels {
				values = {
				  "foo" = "bar",
				  "bar" = "baz",
				}
			}
			stage.structured_metadata {
				values = {
				  "foo" = "",
				  "bar" = "",
				}
			}

			stage.structured_metadata_drop {
				values = ["foo"]
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "", now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					StructuredMetadata: push.LabelsAdapter{{Name: "bar", Value: "baz"}},
				}),
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

func TestValidateStructuredMetadataDropConfig(t *testing.T) {
	type testCase struct {
		name   string
		config StructuredMetadataDropConfig
		err    error
	}

	tests := []testCase{
		{
			name:   "empty config",
			config: StructuredMetadataDropConfig{},
			err:    errEmptyStructuredMetadataDropStageConfig,
		},
		{
			name:   "with values",
			config: StructuredMetadataDropConfig{Values: []string{"1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, validateStructuredMetadataDropConfig(tt.config), tt.err)
		})
	}
}
