package stages

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
)

var (
	criTestTimeStr = "2019-01-01T01:00:00.000000001Z"
	criTestTime, _ = time.Parse(time.RFC3339Nano, criTestTimeStr)
	criTestTime2   = time.Now()
)

func TestCRI(t *testing.T) {
	tests := map[string]struct {
		entry          string
		expectedLine   string
		ts             time.Time
		expectedTs     time.Time
		expectedLabels model.LabelSet
	}{
		"happy path": {
			criTestTimeStr + " stderr F message",
			"message",
			time.Now(),
			criTestTime,
			model.LabelSet{
				"stream": "stderr",
			},
		},
		"multi line pass": {
			criTestTimeStr + " stderr F message\nmessage2",
			"message\nmessage2",
			time.Now(),
			criTestTime,
			model.LabelSet{
				"stream": "stderr",
			},
		},
		"invalid timestamp": {
			"3242 stderr F message",
			"message",
			criTestTime2,
			criTestTime2,
			model.LabelSet{
				"stream": "stderr",
			},
		},
		"invalid line": {
			"i'm invalid!!!",
			"i'm invalid!!!",
			criTestTime2,
			criTestTime2,
			model.LabelSet{},
		},
		"gracefully handle without flag": {
			entry:          "something stderr looks like it could be cri",
			expectedLine:   "looks like it could be cri",
			ts:             criTestTime2,
			expectedTs:     criTestTime2,
			expectedLabels: model.LabelSet{"stream": "stderr"},
		},
	}

	for tName, tt := range tests {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()

			p, err := NewCRI(log.NewNopLogger(), DefaultCRIConfig, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)

			out := processEntries(p, newEntry(nil, model.LabelSet{}, tt.entry, tt.ts))[0]
			assert.EqualValues(t, tt.expectedLabels, out.Labels)
			assert.Equal(t, tt.expectedLine, out.Line, "did not receive expected log entry")
			assert.Equal(t, tt.expectedTs.Unix(), out.Timestamp.Unix())
		})
	}
}

func TestCRI_tags(t *testing.T) {
	type testEntry struct {
		labels model.LabelSet
		line   string
	}

	type testCase struct {
		name                       string
		expected                   []string
		maxPartialLines            int
		maxPartialLineSize         uint64
		maxPartialLineSizeTruncate bool
		entries                    []testEntry
	}

	cases := []testCase{
		{
			name:            "tag F",
			maxPartialLines: 100,
			entries: []testEntry{
				{line: "2019-05-07T18:57:50.904275087+00:00 stdout F some full line", labels: model.LabelSet{"foo": "bar"}},
				{line: "2019-05-07T18:57:55.904275087+00:00 stdout F log", labels: model.LabelSet{"foo": "bar"}},
			},
			expected: []string{"some full line", "log"},
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
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := CRIConfig{
				MaxPartialLines:            tt.maxPartialLines,
				MaxPartialLineSize:         tt.maxPartialLineSize,
				MaxPartialLineSizeTruncate: tt.maxPartialLineSizeTruncate,
			}
			p, err := NewCRI(log.NewNopLogger(), cfg, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)

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
		})
	}
}

var (
	benchCRITime  = time.Now()
	benchCRIEntry Entry
	benchCRILine  = "2019-01-01T01:00:00.000000001Z stderr F my cool message yay\n test"
)

func BenchmarkCRI(b *testing.B) {
	p, _ := NewCRI(log.NewNopLogger(), DefaultCRIConfig, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
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
