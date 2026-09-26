package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestStaticLabelsTest(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		cfg      StaticLabelsConfig
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "add static label",
			cfg: StaticLabelsConfig{Values: map[string]*string{
				"staticLabel": new("val"),
			}},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"testLabel": "testValue",
				}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"testLabel": "testValue",
				}, model.LabelSet{
					"testLabel":   "testValue",
					"staticLabel": "val",
				}, "", now),
			},
		},
		{
			name: "add static label with empty value",
			cfg: StaticLabelsConfig{Values: map[string]*string{
				"staticLabel": nil,
			}},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"testLabel": "testValue",
				}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"testLabel": "testValue",
				}, model.LabelSet{
					"testLabel": "testValue",
				}, "", now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{StaticLabelsConfig: &tt.cfg}}, tt.entries, tt.expected)
		})
	}
}

func TestValidateStaticLabelsConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		expectErr bool
	}{
		{
			name:   "valid",
			config: `values = { "staticLabel" = "val" }`,
		},
		{
			name:   "null value is skipped",
			config: `values = { "staticLabel" = null }`,
		},
		{
			name:   "empty value is skipped",
			config: `values = { "staticLabel" = "" }`,
		},
		{
			name:      "invalid label value",
			config:    `values = { "staticLabel" = "\xfd" }`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg StaticLabelsConfig
			err := syntax.Unmarshal([]byte(tt.config), &cfg)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
