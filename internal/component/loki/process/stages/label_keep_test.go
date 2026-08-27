package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

func TestLabelKeepStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		cfg      LabelKeepConfig
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "allow single label",
			cfg:  LabelKeepConfig{Values: []string{"testLabel1"}},
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
				}, "", now),
			},
		},
		{
			name: "allow multiple labels",
			cfg:  LabelKeepConfig{Values: []string{"testLabel1", "testLabel2"}},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
					"testLabel3": "testValue",
				}, "", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
					"testLabel3": "testValue",
				}, model.LabelSet{
					"testLabel1": "testValue",
					"testLabel2": "testValue",
				}, "", now),
			},
		},
		{
			name: "allow non-existing label",
			cfg:  LabelKeepConfig{Values: []string{"foobar"}},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{LabelKeepConfig: &tt.cfg}}, tt.entries, tt.expected)
		})
	}
}
