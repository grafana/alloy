package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
)

func TestBatch_MaxStreams(t *testing.T) {
	maxStream := 2

	var inputEntries = []loki.Entry{
		{Labels: model.LabelSet{"app": "app-1"}, Entry: push.Entry{Timestamp: time.Unix(4, 0).UTC(), Line: "line4"}},
		{Labels: model.LabelSet{"app": "app-2"}, Entry: push.Entry{Timestamp: time.Unix(5, 0).UTC(), Line: "line5"}},
		{Labels: model.LabelSet{"app": "app-3"}, Entry: push.Entry{Timestamp: time.Unix(6, 0).UTC(), Line: "line6"}},
		{Labels: model.LabelSet{"app": "app-4"}, Entry: push.Entry{Timestamp: time.Unix(6, 0).UTC(), Line: "line6"}},
	}

	b := newBatch(maxStream, int(1*units.MiB))

	errCount := 0
	for _, entry := range inputEntries {
		err := b.add(entry, 0)
		if err != nil {
			errCount++
			assert.ErrorIs(t, err, errMaxStreamsLimitExceeded)
		}
	}
	assert.Equal(t, errCount, 2)
}

func TestBatch_MaxSize(t *testing.T) {
	entry := loki.Entry{Labels: model.LabelSet{"app": "app-1"}, Entry: push.Entry{Timestamp: time.Unix(4, 0).UTC(), Line: "line4"}}
	b := newBatch(0, 1)
	require.NoError(t, b.add(entry, 0))
	require.ErrorIs(t, b.add(entry, 0), errBatchSizeReached)
}

func TestBatch_add(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		entries           []loki.Entry
		expectedSizeBytes int
	}

	batchSize := func(entries ...loki.Entry) int {
		var size int
		for _, e := range entries {
			size += e.Size()
		}
		return size
	}

	tests := []testCase{
		{
			name:              "empty batch",
			entries:           []loki.Entry{},
			expectedSizeBytes: 0,
		},
		{
			name: "single stream with single log entry",
			entries: []loki.Entry{
				{Labels: model.LabelSet{}, Entry: logEntries[0].Entry},
			},
			expectedSizeBytes: batchSize(logEntries[0]),
		},
		{
			name: "single stream with multiple log entries",
			entries: []loki.Entry{
				{Labels: model.LabelSet{}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{}, Entry: logEntries[1].Entry},
				{Labels: model.LabelSet{}, Entry: logEntries[7].Entry},
			},
			expectedSizeBytes: batchSize(logEntries[0], logEntries[1], logEntries[7]),
		},
		{
			name: "multiple streams with multiple log entries",
			entries: []loki.Entry{
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[1].Entry},
				{Labels: model.LabelSet{"type": "b"}, Entry: logEntries[2].Entry},
			},
			expectedSizeBytes: batchSize(logEntries[0], logEntries[1], logEntries[0]),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBatch(0, int(1*units.MiB))
			for _, entry := range tt.entries {
				err := b.add(entry, 0)
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedSizeBytes, b.size)
		})
	}
}

func TestBatchHashCollisions(t *testing.T) {
	b := newBatch(0, int(1*units.MiB))

	ls1 := model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}
	ls2 := model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}

	require.False(t, ls1.Equal(ls2))
	require.Equal(t, ls1.FastFingerprint(), ls2.FastFingerprint())

	const entriesPerLabel = 10

	for i := range entriesPerLabel {
		_ = b.add(loki.Entry{Labels: ls1, Entry: push.Entry{Timestamp: time.Now(), Line: fmt.Sprintf("line %d", i)}}, 0)

		_ = b.add(loki.Entry{Labels: ls2, Entry: push.Entry{Timestamp: time.Now(), Line: fmt.Sprintf("line %d", i)}}, 0)
	}

	// make sure that colliding labels are stored properly as independent streams
	req, entries := b.req, b.entriesTotal
	assert.Len(t, req.Streams, 2)
	assert.Equal(t, 2*entriesPerLabel, entries)

	if req.Streams[0].Labels == ls1.String() {
		assert.Equal(t, ls1.String(), req.Streams[0].Labels)
		assert.Equal(t, ls2.String(), req.Streams[1].Labels)
	} else {
		assert.Equal(t, ls2.String(), req.Streams[0].Labels)
		assert.Equal(t, ls1.String(), req.Streams[1].Labels)
	}
}

func BenchmarkBatch_Add(b *testing.B) {
	const maxSize = 1 << 20
	entries := make([]loki.Entry, 64)
	for i := range 64 {
		entries[i] = loki.Entry{
			Labels: model.LabelSet{"stream": model.LabelValue(fmt.Sprintf("s%d", i))},
			Entry:  push.Entry{Timestamp: time.Now(), Line: "log line"},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		batch := newBatch(0, maxSize)
		for i := range 64 {
			_ = batch.add(entries[i%64], 0)
		}
	}
}

// store the result to a package level variable
// so the compiler cannot eliminate the Benchmark itself.
var result string

func BenchmarkLabelsMapToString(b *testing.B) {
	labelSet := make(model.LabelSet)
	labelSet["label"] = "value"
	labelSet["label1"] = "value2"
	labelSet["label2"] = "value3"
	labelSet["__tenant_id__"] = "another_value"

	var r string
	for b.Loop() {
		// store in r prevent the compiler eliminating the function call.
		r = labelsMapToString(labelSet)
	}
	result = r
}

func TestLabelsMapToString(t *testing.T) {
	tests := []struct {
		name     string
		input    model.LabelSet
		expected string
	}{
		{
			name:     "empty label set",
			input:    model.LabelSet{},
			expected: "{}",
		},
		{
			name:     "single label",
			input:    model.LabelSet{"app": "my-app"},
			expected: `{app="my-app"}`,
		},
		{
			name:     "multiple labels",
			input:    model.LabelSet{"app": "my-app", "env": "prod"},
			expected: `{app="my-app", env="prod"}`,
		},
		{
			name:     "labels with reserved labels",
			input:    model.LabelSet{"app": "my-app", "__meta_label_abc": "meta-abc", "__meta_label_def": "meta-def", "abc": "123"},
			expected: `{abc="123", app="my-app"}`,
		},
		{
			name:     "only reserved labels",
			input:    model.LabelSet{"__meta_label_abc": "meta-abc", "__meta_label_def": "meta-def"},
			expected: "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := labelsMapToString(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
