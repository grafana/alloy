package client

import (
	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
)

// encode marshals request to a snappy-compressed push request using the
// given buffers, and returns the encoded bytes and any error.
// If the request does not fit in protoBuf or the compressed output does not fit in
// snappyBuf, new buffers are allocated and the caller's buffers are
// not reused.
// protoBuf and snappyBuf must not overlap.
func encode(r *push.PushRequest, size int, protoBuf, snappyBuf []byte) ([]byte, error) {
	// Note: Just a safeguard in case the passed-in buffer is too small so that
	// MarshalToSizedBuffer doesn't panic.
	if size > len(protoBuf) {
		protoBuf = make([]byte, size)
	}

	n, err := r.MarshalToSizedBuffer(protoBuf[:size])
	if err != nil {
		return nil, err
	}

	// NOTE: if buffer is too small to hold compressed data snappy.Encode will
	// allocate a new one and the passed in buffer is not reused.
	return snappy.Encode(snappyBuf, protoBuf[:n]), nil
}
