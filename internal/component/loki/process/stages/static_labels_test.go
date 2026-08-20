package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
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
				"staticLabel": ptr("val"),
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
