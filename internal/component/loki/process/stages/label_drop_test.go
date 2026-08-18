package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

func TestLabelDropStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		cfg      LabelDropConfig
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "drop one label",
			cfg:  LabelDropConfig{Values: []string{"testLabel1"}},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, model.LabelSet{
					"testLabel2": "testValue",
				}, "", now),
			},
		},
		{
			name: "drop two labels",
			cfg:  LabelDropConfig{Values: []string{"testLabel1", "testLabel2"}},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, model.LabelSet{}, "", now),
			},
		},
		{
			name: "drop non-existing label",
			cfg:  LabelDropConfig{Values: []string{"foobar"}},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, model.LabelSet{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, "", now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{LabelDropConfig: &tt.cfg}}, tt.entries, tt.expected, "")
		})
	}
}
