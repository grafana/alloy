package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/golang/snappy"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
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
			maxSize := int(1 * units.MiB)
			b := newBatch(0, maxSize)
			for _, e := range tt.entries {
				require.NoError(t, b.add(e, 0))
			}

			r, entries := b.request()
			require.Equal(t, tt.expectedEntriesCount, entries)

			var (
				protoBuf  = make([]byte, maxSize)
				snappyBuf = make([]byte, snappy.MaxEncodedLen(maxSize))
			)

			_, err := encode(r, r.Size(), protoBuf, snappyBuf)
			require.NoError(t, err)
		})
	}
}

func TestEncodeSmallBuffers(t *testing.T) {
	b := newBatch(0, int(1*units.MiB))
	require.NoError(t, b.add(loki.Entry{}, 0))

	r, entries := b.request()
	require.Equal(t, 1, entries)

	// Pass buffers that are too small to hold data.
	_, err := encode(r, r.Size(), []byte{}, []byte{})
	require.NoError(t, err)
}

var encodedBuf []byte

func BenchmarkBatch_Encode(b *testing.B) {
	const maxSize = 1 << 20
	batch := newBatch(0, maxSize)
	for i := range 64 {
		entry := loki.Entry{
			Labels: model.LabelSet{"stream": model.LabelValue(fmt.Sprintf("s%d", i))},
			Entry:  push.Entry{Timestamp: time.Now(), Line: "log line"},
		}
		err := batch.add(entry, 0)
		require.NoError(b, err)
	}

	r, _ := batch.request()
	size := r.Size()

	var (
		protoBuf  = make([]byte, size)
		snappyBuf = make([]byte, snappy.MaxEncodedLen(size))
	)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		encodedBuf, _ = encode(r, size, protoBuf, snappyBuf)
	}
}
