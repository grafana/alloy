package util_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/loki/util"
)

func TestParseProtoReader(t *testing.T) {
	// 42 bytes compressed and 71 uncompressed
	req := push.PushRequest{
		Streams: []push.Stream{
			{
				Labels: `{foo="foo"}`,
				Entries: []push.Entry{
					{Line: "foo foo foo foo foo foo foo foo foo foo"},
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
		{"rawSnappy", util.RawSnappy, 71, false, false},
		{"noCompression", util.NoCompression, 71, false, false},
		{"too big rawSnappy", util.RawSnappy, 10, true, false},
		{"too big noCompression", util.NoCompression, 10, true, false},

		{"bytesbuffer rawSnappy", util.RawSnappy, 71, false, true},
		{"bytesbuffer noCompression", util.NoCompression, 71, false, true},
		{"bytesbuffer too big rawSnappy", util.RawSnappy, 10, true, true},
		{"bytesbuffer too big noCompression", util.NoCompression, 10, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			assert.Nil(t, util.SerializeProto(w, &req, tt.compression))

			reader := w.Result().Body
			if tt.useBytesBuffer {
				buf := bytes.Buffer{}
				_, err := buf.ReadFrom(reader)
				assert.Nil(t, err)
				reader = bytesBuffered{Buffer: &buf}
			}

			var fromWire push.PushRequest
			err := util.ParseProtoReader(context.Background(), reader, 0, tt.maxSize, &fromWire, tt.compression)
			if tt.expectErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, req, fromWire)
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
