package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestBatch_MaxStreams(t *testing.T) {
	maxStream := 2

	var inputEntries = []loki.Entry{
		{Labels: model.LabelSet{"app": "app-1"}, Entry: push.Entry{Timestamp: time.Unix(4, 0).UTC(), Line: "line4"}},
		{Labels: model.LabelSet{"app": "app-2"}, Entry: push.Entry{Timestamp: time.Unix(5, 0).UTC(), Line: "line5"}},
		{Labels: model.LabelSet{"app": "app-3"}, Entry: push.Entry{Timestamp: time.Unix(6, 0).UTC(), Line: "line6"}},
		{Labels: model.LabelSet{"app": "app-4"}, Entry: push.Entry{Timestamp: time.Unix(6, 0).UTC(), Line: "line6"}},
	}

	b := newBatch(maxStream)

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

func TestBatch_add(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		inputEntries      []loki.Entry
		expectedSizeBytes int
	}{
		"empty batch": {
			inputEntries:      []loki.Entry{},
			expectedSizeBytes: 0,
		},
		"single stream with single log entry": {
			inputEntries: []loki.Entry{
				{Labels: model.LabelSet{}, Entry: logEntries[0].Entry},
			},
			expectedSizeBytes: len(logEntries[0].Entry.Line),
		},
		"single stream with multiple log entries": {
			inputEntries: []loki.Entry{
				{Labels: model.LabelSet{}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{}, Entry: logEntries[1].Entry},
				{Labels: model.LabelSet{}, Entry: logEntries[7].Entry},
			},
			expectedSizeBytes: entrySize(logEntries[0].Entry) + entrySize(logEntries[0].Entry) + entrySize(logEntries[7].Entry),
		},
		"multiple streams with multiple log entries": {
			inputEntries: []loki.Entry{
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[1].Entry},
				{Labels: model.LabelSet{"type": "b"}, Entry: logEntries[2].Entry},
			},
			expectedSizeBytes: len(logEntries[0].Entry.Line) + len(logEntries[1].Entry.Line) + len(logEntries[2].Entry.Line),
		},
	}

	for testName, testData := range tests {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			b := newBatch(0)

			for _, entry := range testData.inputEntries {
				err := b.add(entry, 0)
				assert.NoError(t, err)
			}

			assert.Equal(t, testData.expectedSizeBytes, b.sizeBytes())
		})
	}
}

func TestBatch_encode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		inputBatch           *batch
		expectedEntriesCount int
	}{
		"empty batch": {
			inputBatch:           newBatch(0),
			expectedEntriesCount: 0,
		},
		"single stream with single log entry": {
			inputBatch: func() *batch {
				b := newBatch(0)
				_ = b.add(loki.Entry{Labels: model.LabelSet{}, Entry: logEntries[0].Entry}, 0)
				return b
			}(),
			expectedEntriesCount: 1,
		},
		"single stream with multiple log entries": {
			inputBatch: func() *batch {
				b := newBatch(0)
				_ = b.add(loki.Entry{Labels: model.LabelSet{}, Entry: logEntries[0].Entry}, 0)
				_ = b.add(loki.Entry{Labels: model.LabelSet{}, Entry: logEntries[1].Entry}, 0)
				return b
			}(),
			expectedEntriesCount: 2,
		},
		"multiple streams with multiple log entries": {
			inputBatch: func() *batch {
				b := newBatch(0)
				_ = b.add(loki.Entry{Labels: model.LabelSet{}, Entry: logEntries[0].Entry}, 0)
				_ = b.add(loki.Entry{Labels: model.LabelSet{}, Entry: logEntries[1].Entry}, 0)
				_ = b.add(loki.Entry{Labels: model.LabelSet{}, Entry: logEntries[2].Entry}, 0)
				return b
			}(),
			expectedEntriesCount: 3,
		},
	}

	for testName, testData := range tests {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			_, entriesCount, err := testData.inputBatch.encode()
			require.NoError(t, err)
			assert.Equal(t, testData.expectedEntriesCount, entriesCount)
		})
	}
}

func TestHashCollisions(t *testing.T) {
	b := newBatch(0)

	ls1 := model.LabelSet{"app": "l", "uniq0": "0", "uniq1": "1"}
	ls2 := model.LabelSet{"app": "m", "uniq0": "1", "uniq1": "1"}

	require.False(t, ls1.Equal(ls2))
	require.Equal(t, ls1.FastFingerprint(), ls2.FastFingerprint())

	const entriesPerLabel = 10

	for i := 0; i < entriesPerLabel; i++ {
		_ = b.add(loki.Entry{Labels: ls1, Entry: push.Entry{Timestamp: time.Now(), Line: fmt.Sprintf("line %d", i)}}, 0)
		_ = b.add(loki.Entry{Labels: ls2, Entry: push.Entry{Timestamp: time.Now(), Line: fmt.Sprintf("line %d", i)}}, 0)
	}

	// make sure that colliding labels are stored properly as independent streams
	req, entries := b.createPushRequest()
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

// store the result to a package level variable
// so the compiler cannot eliminate the Benchmark itself.
var result string

func BenchmarkLabelsMapToString(b *testing.B) {
	labelSet := make(model.LabelSet)
	labelSet["label"] = "value"
	labelSet["label1"] = "value2"
	labelSet["label2"] = "value3"
	labelSet["__tenant_id__"] = "another_value"

	b.ResetTimer()
	var r string
	for i := 0; i < b.N; i++ {
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := labelsMapToString(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
