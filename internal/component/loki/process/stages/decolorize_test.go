package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

func TestDecolorizeStage(t *testing.T) {
	type testCase struct {
		name     string
		entries  []Entry
		expected []Entry
	}

	now := time.Now()

	tests := []testCase{
		{
			name: "successfully run pipeline on non-colored text",
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "sample text", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "sample text", now),
			},
		},
		{
			name: "successfully run pipeline on colored text",
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "\033[0;32mgreen\033[0m \033[0;31mred\033[0m", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "green red", now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runPipelineTest(t, []StageConfig{{DecolorizeConfig: &DecolorizeConfig{}}}, tt.entries, tt.expected, "")
		})
	}
}
