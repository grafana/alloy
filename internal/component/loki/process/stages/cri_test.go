package stages

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestCRIStage(t *testing.T) {
	var (
		criTestTimeStr = "2019-01-01T01:00:00.000000001Z"
		criTestTime, _ = time.Parse(time.RFC3339Nano, criTestTimeStr)
		criTestTime2   = time.Now()

		tagFTime1Str = "2019-05-07T18:57:50.904275087+00:00"
		tagFTime1, _ = time.Parse(time.RFC3339Nano, tagFTime1Str)
		tagFTime2Str = "2019-05-07T18:57:55.904275087+00:00"
		tagFTime2, _ = time.Parse(time.RFC3339Nano, tagFTime2Str)
	)

	type testCase struct {
		name                        string
		entries                     []Entry
		expected                    []Entry
		cfg                         CRIConfig
		expectedPartialLinesFlushed int
		expectedLinesTruncated      int
	}

	tests := []testCase{
		{
			name: "full line",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, criTestTimeStr+" stderr F message", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"flags": "F", "stream": "stderr", "content": "message", "time": criTestTimeStr},
					model.LabelSet{"stream": "stderr"},
					"message",
					criTestTime,
				),
			},
		},
		{
			name: "full line multiline",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, criTestTimeStr+" stderr F message\nmessage2", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"flags": "F", "stream": "stderr", "content": "message\nmessage2", "time": criTestTimeStr},
					model.LabelSet{"stream": "stderr"},
					"message\nmessage2",
					criTestTime,
				),
			},
		},
		{
			name: "with invalid timestamp",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "3242 stderr F message", criTestTime2),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"flags": "F", "stream": "stderr", "content": "message", "time": "3242"},
					model.LabelSet{"stream": "stderr"},
					"message",
					criTestTime2,
				),
			},
		},
		{
			name: "with invalid line",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm invalid!!!", criTestTime2),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm invalid!!!", criTestTime2),
			},
		},
		{
			name: "with invalid line",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm invalid!!!", criTestTime2),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm invalid!!!", criTestTime2),
			},
		},
		{
			name: "tag F",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout F some full line", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "some full line", "time": tagFTime1Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"some full line",
					tagFTime1,
				),
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"log",
					tagFTime2,
				),
			},
		},
		{
			name: "tag P multi-stream",
			cfg:  defaultCRIConfig,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime1Str+" stdout P partial line 2 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log finished", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime2Str+" stdout F another full log", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"partial line 1 log finished",
					tagFTime2,
				),
				newEntry(
					map[string]any{"foo": "bar2", "flags": "F", "stream": "stdout", "content": "another full log", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar2", "stream": "stdout"},
					"partial line 2 another full log",
					tagFTime2,
				),
			},
		},
		{
			name: "tag P single stream",
			cfg:  CRIConfig{MaxPartialLines: 3},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 2 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 3 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 4 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log finished", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F another full log", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"partial line 1 partial line 2 partial line 3 partial line 4 log finished",
					tagFTime2,
				),
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "another full log", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"another full log",
					tagFTime2,
				),
			},
		},
		{
			name: "tag P multi-stream with truncation",
			cfg:  CRIConfig{MaxPartialLines: 100, MaxPartialLineSizeTruncate: true, MaxPartialLineSize: 11},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime1Str+" stdout P partial", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar"}, tagFTime2Str+" stdout F log finished", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"foo": "bar2"}, tagFTime2Str+" stdout F full", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"foo": "bar", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar", "stream": "stdout"},
					"partial lin",
					tagFTime2,
				),
				newEntry(
					map[string]any{"foo": "bar2", "flags": "F", "stream": "stdout", "content": "full", "time": tagFTime2Str},
					model.LabelSet{"foo": "bar2", "stream": "stdout"},
					"partialfull",
					tagFTime2,
				),
			},
			expectedLinesTruncated: 2,
		},
		{
			name: "tag P multi-stream with maxPartialLines exceeded",
			cfg:  CRIConfig{MaxPartialLines: 3},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"label1": "val1", "label2": "val2"}, tagFTime1Str+" stdout P partial line 1 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"label1": "val1"}, tagFTime1Str+" stdout P partial line 2 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"label1": "val1", "label2": "val2"}, tagFTime1Str+" stdout P partial line 3 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"label1": "val3"}, tagFTime1Str+" stdout P partial line 4 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"label1": "val4"}, tagFTime1Str+" stdout P partial line 5 ", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"label1": "val1", "label2": "val2"}, tagFTime2Str+" stdout F log finished", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"label1": "val3"}, tagFTime2Str+" stdout F another full log", time.Now()),
				newEntry(map[string]any{}, model.LabelSet{"label1": "val4"}, tagFTime2Str+" stdout F yet an another full log", time.Now()),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"label1": "val1", "label2": "val2", "flags": "P", "stream": "stdout", "content": "partial line 3 ", "time": tagFTime1Str},
					model.LabelSet{"label1": "val1", "label2": "val2", "stream": "stdout"},
					"partial line 1 partial line 3 ",
					tagFTime1,
				),
				newEntry(
					map[string]any{"label1": "val1", "flags": "P", "stream": "stdout", "content": "partial line 2 ", "time": tagFTime1Str},
					model.LabelSet{"label1": "val1", "stream": "stdout"},
					"partial line 2 ",
					tagFTime1,
				),
				newEntry(
					map[string]any{"label1": "val3", "flags": "P", "stream": "stdout", "content": "partial line 4 ", "time": tagFTime1Str},
					model.LabelSet{"label1": "val3", "stream": "stdout"},
					"partial line 4 ",
					tagFTime1,
				),
				newEntry(
					map[string]any{"label1": "val1", "label2": "val2", "flags": "F", "stream": "stdout", "content": "log finished", "time": tagFTime2Str},
					model.LabelSet{"label1": "val1", "label2": "val2", "stream": "stdout"},
					"log finished",
					tagFTime2,
				),
				newEntry(
					map[string]any{"label1": "val3", "flags": "F", "stream": "stdout", "content": "another full log", "time": tagFTime2Str},
					model.LabelSet{"label1": "val3", "stream": "stdout"},
					"another full log",
					tagFTime2,
				),
				newEntry(
					map[string]any{"label1": "val4", "flags": "F", "stream": "stdout", "content": "yet an another full log", "time": tagFTime2Str},
					model.LabelSet{"label1": "val4", "stream": "stdout"},
					"partial line 5 yet an another full log",
					tagFTime2,
				),
			},
			expectedPartialLinesFlushed: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expectedMetrics := fmt.Sprintf(`
# HELP loki_process_cri_lines_truncated_total A count of lines that were truncated due to the max_partial_line_size limit
# TYPE loki_process_cri_lines_truncated_total counter
loki_process_cri_lines_truncated_total %d
# HELP loki_process_cri_partial_lines_flushed_total A count of partial lines that were flushed prematurely due to the max_partial_lines limit being exceeded
# TYPE loki_process_cri_partial_lines_flushed_total counter
loki_process_cri_partial_lines_flushed_total %d
`, tt.expectedLinesTruncated, tt.expectedPartialLinesFlushed)

			runPipelineTest(t, []StageConfig{{CRIConfig: &tt.cfg}}, tt.entries, tt.expected, expectedMetrics)
		})
	}
}

func TestCRI_tags(t *testing.T) {
	type testEntry struct {
		labels model.LabelSet
		line   string
	}

	type testCase struct {
		name                        string
		expected                    []string
		maxPartialLines             int
		maxPartialLineSize          uint64
		maxPartialLineSizeTruncate  bool
		entries                     []testEntry
		expectedPartialLinesFlushed int // expected value of the partial lines flushed metric
		expectedLinesTruncated      int // expected value of the lines truncated metric
	}

	cases := []testCase{
		{
			name:            "tag F",
			maxPartialLines: 100,
			entries: []testEntry{
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout F some full line", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F log", labels: model.LabelSet{"foo": "bar"}},
			},
			expected:                    []string{"some full line", "log"},
			expectedPartialLinesFlushed: 0,
			expectedLinesTruncated:      0,
		},
		{
			name:            "tag P multi-stream",
			maxPartialLines: 100,
			entries: []testEntry{
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 1 ", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 2 ", labels: model.LabelSet{"foo": "bar2"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F log finished", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F another full log", labels: model.LabelSet{"foo": "bar2"}},
			},
			expected: []string{
				"partial line 1 log finished",     // belongs to stream `{foo="bar"}`
				"partial line 2 another full log", // belongs to stream `{foo="bar2"}
			},
			expectedPartialLinesFlushed: 0,
			expectedLinesTruncated:      0,
		},
		{
			name: "tag P multi-stream with maxPartialLines exceeded",
			entries: []testEntry{
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 1 ", labels: model.LabelSet{"label1": "val1", "label2": "val2"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 2 ", labels: model.LabelSet{"label1": "val1"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 3 ", labels: model.LabelSet{"label1": "val1", "label2": "val2"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 4 ", labels: model.LabelSet{"label1": "val3"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 5 ", labels: model.LabelSet{"label1": "val4"}}, // exceeded maxPartialLines as already 3 streams in flight.
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F log finished", labels: model.LabelSet{"label1": "val1", "label2": "val2"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F another full log", labels: model.LabelSet{"label1": "val3"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F yet an another full log", labels: model.LabelSet{"label1": "val4"}},
			},
			maxPartialLines: 3,
			expected: []string{
				"partial line 1 partial line 3 ",
				"partial line 2 ",
				"partial line 4 ",
				"log finished",
				"another full log",
				"partial line 5 yet an another full log",
			},
			expectedPartialLinesFlushed: 3, // 3 partial lines were flushed when limit was exceeded
			expectedLinesTruncated:      0,
		},
		{
			name: "tag P single stream",
			entries: []testEntry{
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 1 ", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 2 ", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 3 ", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 4 ", labels: model.LabelSet{"foo": "bar"}}, // this exceeds the `MaxPartialLinesSize` of 3
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F log finished", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F another full log", labels: model.LabelSet{"foo": "bar"}},
			},
			maxPartialLines: 3,
			expected: []string{
				"partial line 1 partial line 2 partial line 3 partial line 4 log finished",
				"another full log",
			},
			expectedPartialLinesFlushed: 0, // single stream, no flush due to limit (partial lines merge within same stream)
			expectedLinesTruncated:      0,
		},
		{
			name: "tag P multi-stream with truncation",
			entries: []testEntry{
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial line 1 ", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout P partial", labels: model.LabelSet{"foo": "bar2"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F log finished", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F full", labels: model.LabelSet{"foo": "bar2"}},
			},
			maxPartialLines:            100,
			maxPartialLineSizeTruncate: true,
			maxPartialLineSize:         11,
			expected: []string{
				"partial lin",
				"partialfull",
			},
			expectedPartialLinesFlushed: 0,
			expectedLinesTruncated:      2, // 2 lines were truncated due to max_partial_line_size
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			cfg := CRIConfig{
				MaxPartialLines:            tt.maxPartialLines,
				MaxPartialLineSize:         tt.maxPartialLineSize,
				MaxPartialLineSizeTruncate: tt.maxPartialLineSizeTruncate,
			}
			p := newCRIStage(logging.NewSlogNop(), cfg, registry, featuregate.StabilityGenerallyAvailable, nil)
			got := make([]string, 0)

			for _, entry := range tt.entries {
				out := processEntries(p, newEntry(nil, entry.labels, entry.line, time.Now()))
				if len(out) > 0 {
					for _, en := range out {
						got = append(got, en.Line)
					}
				}
			}

			expectedMap := make(map[string]bool)
			for _, v := range tt.expected {
				expectedMap[v] = true
			}

			gotMap := make(map[string]bool)
			for _, v := range got {
				gotMap[v] = true
			}

			assert.Equal(t, expectedMap, gotMap)

			// Verify the metrics
			expectedMetrics := fmt.Sprintf(`
# HELP loki_process_cri_lines_truncated_total A count of lines that were truncated due to the max_partial_line_size limit
# TYPE loki_process_cri_lines_truncated_total counter
loki_process_cri_lines_truncated_total %d
# HELP loki_process_cri_partial_lines_flushed_total A count of partial lines that were flushed prematurely due to the max_partial_lines limit being exceeded
# TYPE loki_process_cri_partial_lines_flushed_total counter
loki_process_cri_partial_lines_flushed_total %d
`, tt.expectedLinesTruncated, tt.expectedPartialLinesFlushed)
			require.NoError(t, testutil.GatherAndCompare(registry, strings.NewReader(expectedMetrics)))
		})
	}
}

var (
	benchCRITime  = time.Now()
	benchCRIEntry Entry
	benchCRILine  = "2019-01-01T01:00:00.000000001Z stderr F my cool message yay\n test"
)

func BenchmarkCRI(b *testing.B) {
	p := newCRIStage(logging.NewSlogNop(), defaultCRIConfig, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable, nil)
	e := newEntry(nil, model.LabelSet{}, benchCRILine, benchCRITime)
	in := make(chan Entry)
	out := p.Run(in)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		in <- e
		benchCRIEntry = <-out
	}
}
