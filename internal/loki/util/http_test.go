package util_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/loki/v3/pkg/logproto"
)

func TestParseProtoReader(t *testing.T) {
	// 47 bytes compressed and 53 uncompressed
	req := &logproto.PreallocWriteRequest{
		WriteRequest: logproto.WriteRequest{
			Timeseries: []logproto.PreallocTimeseries{
				{
					TimeSeries: &logproto.TimeSeries{
						Labels: []logproto.LabelAdapter{
							{Name: "foo", Value: "bar"},
						},
						Samples: []logproto.LegacySample{
							{Value: 10, TimestampMs: 1},
							{Value: 20, TimestampMs: 2},
							{Value: 30, TimestampMs: 3},
						},
					},
				},
			},
		},
	}

	for _, tt := range []struct {
		name           string
		compression    util.CompressionType
		maxSize        int
		expectErr      bool
		useBytesBuffer bool
	}{
		{"rawSnappy", util.RawSnappy, 53, false, false},
		{"noCompression", util.NoCompression, 53, false, false},
		{"too big rawSnappy", util.RawSnappy, 10, true, false},
		{"too big decoded rawSnappy", util.RawSnappy, 50, true, false},
		{"too big noCompression", util.NoCompression, 10, true, false},

		{"bytesbuffer rawSnappy", util.RawSnappy, 53, false, true},
		{"bytesbuffer noCompression", util.NoCompression, 53, false, true},
		{"bytesbuffer too big rawSnappy", util.RawSnappy, 10, true, true},
		{"bytesbuffer too big decoded rawSnappy", util.RawSnappy, 50, true, true},
		{"bytesbuffer too big noCompression", util.NoCompression, 10, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			assert.Nil(t, util.SerializeProtoResponse(w, req, tt.compression))
			var fromWire logproto.PreallocWriteRequest

			reader := w.Result().Body
			if tt.useBytesBuffer {
				buf := bytes.Buffer{}
				_, err := buf.ReadFrom(reader)
				assert.Nil(t, err)
				reader = bytesBuffered{Buffer: &buf}
			}

			err := util.ParseProtoReader(context.Background(), reader, 0, tt.maxSize, &fromWire, tt.compression)
			if tt.expectErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, req, &fromWire)
		})
	}
}

type bytesBuffered struct {
	*bytes.Buffer
}

func (b bytesBuffered) Close() error {
	return nil
}

func (b bytesBuffered) BytesBuffer() *bytes.Buffer {
	return b.Buffer
}
