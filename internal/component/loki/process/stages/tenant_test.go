package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestTenantStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "should not set the tenant if the source field is not defined in the extracted map",
			config: `
			stage.tenant {
				source = "tenant_id"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "hello world", now),
			},
		},
		{
			name: "should not override the tenant if the source field is not defined in the extracted map",
			config: `
			stage.tenant {
				source = "tenant_id"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{ReservedLabelTenantID: "foo"}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{ReservedLabelTenantID: "foo"}, model.LabelSet{ReservedLabelTenantID: "foo"}, "hello world", now),
			},
		},
		{
			name: "should set the tenant if the source field is defined in the extracted map",
			config: `
			stage.tenant {
				source = "tenant_id"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"tenant_id": "bar"}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"tenant_id": "bar"}, model.LabelSet{ReservedLabelTenantID: "bar"}, "hello world", now),
			},
		},
		{
			name: "should set the tenant if the label is defined in the label map",
			config: `
			stage.tenant {
				label = "tenant_id"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"tenant_id": "bar"}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"tenant_id": "bar"}, model.LabelSet{"tenant_id": "bar", ReservedLabelTenantID: "bar"}, "hello world", now),
			},
		},
		{
			name: "should override the tenant if the source field is defined in the extracted map",
			config: `
			stage.tenant {
				source = "tenant_id"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"tenant_id": "bar"}, model.LabelSet{ReservedLabelTenantID: "foo"}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"tenant_id": "bar", ReservedLabelTenantID: "foo"}, model.LabelSet{ReservedLabelTenantID: "bar"}, "hello world", now),
			},
		},
		{
			name: "should not set the tenant if the source field data type can't be converted to string",
			config: `
			stage.tenant {
				source = "tenant_id"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"tenant_id": []string{"bar"}}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"tenant_id": []string{"bar"}}, model.LabelSet{}, "hello world", now),
			},
		},
		{
			name: "should set the tenant with the configured static value",
			config: `
			stage.tenant {
				value = "bar"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{ReservedLabelTenantID: "bar"}, "hello world", now),
			},
		},
		{
			name: "should override the tenant with the configured static value",
			config: `
			stage.tenant {
				value = "bar"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{ReservedLabelTenantID: "foo"}, "hello world", now),
			},
			expected: []Entry{
				newEntry(map[string]any{ReservedLabelTenantID: "foo"}, model.LabelSet{ReservedLabelTenantID: "bar"}, "hello world", now),
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

func TestValidateTenantConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		cfg  TenantConfig
		err  error
	}

	tests := []testCase{
		{
			name: "should pass on source config option set",
			cfg:  TenantConfig{Source: "tenant"},
		},
		{
			name: "should pass on value config option set",
			cfg:  TenantConfig{Value: "team-a"},
		},
		{
			name: "should fail on missing source and value",
			cfg:  TenantConfig{},
			err:  errTenantStageEmptyLabelSourceOrValue,
		},
		{
			name: "should fail on empty source",
			cfg:  TenantConfig{Source: ""},
			err:  errTenantStageEmptyLabelSourceOrValue,
		},
		{
			name: "should fail on empty value",
			cfg:  TenantConfig{Value: ""},
			err:  errTenantStageEmptyLabelSourceOrValue,
		},
		{
			name: "should fail on empty label",
			cfg:  TenantConfig{Label: ""},
			err:  errTenantStageEmptyLabelSourceOrValue,
		},
		{
			name: "should fail on both source and value set",
			cfg:  TenantConfig{Source: "tenant", Value: "team-a"},
			err:  errTenantStageConflictingLabelSourceAndValue,
		},
		{
			name: "should fail on both source and label set",
			cfg:  TenantConfig{Source: "tenant", Label: "team-a"},
			err:  errTenantStageConflictingLabelSourceAndValue,
		},
		{
			name: "should fail on both label and value set",
			cfg:  TenantConfig{Label: "tenant", Value: "team-a"},
			err:  errTenantStageConflictingLabelSourceAndValue,
		},
		{
			name: "should fail on all set",
			cfg:  TenantConfig{Label: "tenant", Source: "tenant", Value: "team-a"},
			err:  errTenantStageConflictingLabelSourceAndValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTenantConfig(tt.cfg)
			require.ErrorIs(t, err, tt.err)
		})
	}
}
