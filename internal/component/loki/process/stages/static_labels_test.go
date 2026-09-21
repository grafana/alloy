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

func TestStaticLabelsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  StaticLabelsConfig
		wantErr string
	}{
		{
			name: "valid",
			config: StaticLabelsConfig{Values: map[string]*string{
				"staticLabel": new("val"),
			}},
		},
		{
			name: "nil value is skipped",
			config: StaticLabelsConfig{Values: map[string]*string{
				"staticLabel": nil,
			}},
		},
		{
			name: "empty value is skipped",
			config: StaticLabelsConfig{Values: map[string]*string{
				"staticLabel": new(""),
			}},
		},
		{
			name:    "nil values",
			config:  StaticLabelsConfig{},
			wantErr: errEmptyStaticLabelStageConfig.Error(),
		},
		{
			name: "empty label name",
			config: StaticLabelsConfig{Values: map[string]*string{
				"": new("val"),
			}},
			wantErr: "invalid label name: ",
		},
		{
			name: "invalid label name with nil value",
			config: StaticLabelsConfig{Values: map[string]*string{
				"\xfd": nil,
			}},
			wantErr: "invalid label name: \xfd",
		},
		{
			name: "invalid UTF-8 label value",
			config: StaticLabelsConfig{Values: map[string]*string{
				"staticLabel": new("\xfd"),
			}},
			wantErr: "invalid label value: \xfd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestStaticLabelsConfig_Unmarshal(t *testing.T) {
	var cfg StaticLabelsConfig
	err := syntax.Unmarshal([]byte(`values = { "staticLabel" = "\xfd" }`), &cfg)
	require.EqualError(t, err, "invalid label value: \xfd")
}
