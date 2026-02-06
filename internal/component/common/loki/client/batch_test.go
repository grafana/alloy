package client

import (
	"fmt"
	"testing"
	"time"

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

	type testCase struct {
		name              string
		entries           []loki.Entry
		expectedSizeBytes int
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
			expectedSizeBytes: logEntries[0].Size(),
		},
		{
			name: "single stream with multiple log entries",
			entries: []loki.Entry{
				{Labels: model.LabelSet{}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{}, Entry: logEntries[1].Entry},
				{Labels: model.LabelSet{}, Entry: logEntries[7].Entry},
			},
			expectedSizeBytes: logEntries[0].Size() + logEntries[0].Size() + logEntries[7].Size(),
		},
		{
			name: "multiple streams with multiple log entries",
			entries: []loki.Entry{
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[1].Entry},
				{Labels: model.LabelSet{"type": "b"}, Entry: logEntries[2].Entry},
			},
			expectedSizeBytes: logEntries[0].Size() + logEntries[1].Size() + logEntries[2].Size(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBatch(0)
			for _, entry := range tt.entries {
				err := b.add(entry, 0)
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedSizeBytes, b.sizeBytes())
		})
	}
}

func TestBatch_encode(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		entries              []loki.Entry
		expectedEntriesCount int
	}

	tests := []testCase{
		{
			name:                 "empty batch",
			entries:              []loki.Entry{},
			expectedEntriesCount: 0,
		},
		{
			name: "single stream with single log entry",
			entries: []loki.Entry{
				{Labels: model.LabelSet{}, Entry: logEntries[0].Entry},
			},
			expectedEntriesCount: 1,
		},
		{
			name: "single stream with multiple log entries",
			entries: []loki.Entry{
				{Labels: model.LabelSet{}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{}, Entry: logEntries[1].Entry},
			},
			expectedEntriesCount: 2,
		},
		{
			name: "multiple streams with multiple log entries",
			entries: []loki.Entry{
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[0].Entry},
				{Labels: model.LabelSet{"type": "a"}, Entry: logEntries[1].Entry},
				{Labels: model.LabelSet{"type": "b"}, Entry: logEntries[2].Entry},
			},
			expectedEntriesCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := newBatch(0)
			for _, e := range tt.entries {
				err := b.add(e, 0)
				require.NoError(t, err)
			}

			_, entriesCount, err := b.encode()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedEntriesCount, entriesCount)
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

	for i := range entriesPerLabel {
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
