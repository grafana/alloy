package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	attribute "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const messageSizeLargerErrFmt = "%w than max (%d vs %d)"

var ErrMessageSizeTooLarge = errors.New("message size too large")

const (
	HTTPRateLimited  = "rate_limited"
	HTTPServerError  = "server_error"
	HTTPErrorUnknown = "unknown"
	HTTPClientError  = "client_error"
)

// CompressionType for encoding and decoding requests and responses.
type CompressionType int

// Values for CompressionType
const (
	NoCompression CompressionType = iota
	RawSnappy
)

// ParseProtoReader parses a compressed proto from an io.Reader.
func ParseProtoReader(ctx context.Context, reader io.Reader, expectedSize, maxSize int, req proto.Message, compression CompressionType) error {
	sp := trace.SpanFromContext(ctx)
	sp.AddEvent("util.ParseProtoRequest[start reading]")
	body, err := decompressRequest(reader, expectedSize, maxSize, compression, sp)
	if err != nil {
		return err
	}

	sp.AddEvent("util.ParseProtoRequest[unmarshal]", trace.WithAttributes(attribute.Int("size", len(body))))

	// We re-implement proto.Unmarshal here as it calls XXX_Unmarshal first,
	// which we can't override without upsetting golint.
	req.Reset()
	if u, ok := req.(proto.Unmarshaler); ok {
		err = u.Unmarshal(body)
	} else {
		err = proto.NewBuffer(body).Unmarshal(req)
	}
	if err != nil {
		return err
	}

	return nil
}

func decompressRequest(reader io.Reader, expectedSize, maxSize int, compression CompressionType, sp trace.Span) (body []byte, err error) {
	defer func() {
		if err != nil && len(body) > maxSize {
			err = fmt.Errorf(messageSizeLargerErrFmt, ErrMessageSizeTooLarge, len(body), maxSize)
		}
	}()
	if expectedSize > maxSize {
		return nil, fmt.Errorf(messageSizeLargerErrFmt, ErrMessageSizeTooLarge, expectedSize, maxSize)
	}
	buffer, ok := tryBufferFromReader(reader)
	if ok {
		body, err = decompressFromBuffer(buffer, maxSize, compression, sp)
		return
	}
	body, err = decompressFromReader(reader, expectedSize, maxSize, compression, sp)
	return
}

func decompressFromReader(reader io.Reader, expectedSize, maxSize int, compression CompressionType, sp trace.Span) ([]byte, error) {
	var (
		buf  bytes.Buffer
		body []byte
		err  error
	)
	if expectedSize > 0 {
		buf.Grow(expectedSize + bytes.MinRead) // extra space guarantees no reallocation
	}
	// Read from LimitReader with limit max+1. So if the underlying
	// reader is over limit, the result will be bigger than max.
	reader = io.LimitReader(reader, int64(maxSize)+1)
	switch compression {
	case NoCompression:
		_, err = buf.ReadFrom(reader)
		body = buf.Bytes()
	case RawSnappy:
		_, err = buf.ReadFrom(reader)
		if err != nil {
			return nil, err
		}
		body, err = decompressFromBuffer(&buf, maxSize, RawSnappy, sp)
	}
	return body, err
}

func decompressFromBuffer(buffer *bytes.Buffer, maxSize int, compression CompressionType, sp trace.Span) ([]byte, error) {
	bufBytes := buffer.Bytes()
	if len(bufBytes) > maxSize {
		return nil, fmt.Errorf(messageSizeLargerErrFmt, ErrMessageSizeTooLarge, len(bufBytes), maxSize)
	}
	switch compression {
	case NoCompression:
		return bufBytes, nil
	case RawSnappy:
		sp.AddEvent("util.ParseProtoRequest[decompress]", trace.WithAttributes(
			attribute.Int("size", len(bufBytes)),
		))
		size, err := snappy.DecodedLen(bufBytes)
		if err != nil {
			return nil, err
		}
		if size > maxSize {
			return nil, fmt.Errorf(messageSizeLargerErrFmt, ErrMessageSizeTooLarge, size, maxSize)
		}
		body, err := snappy.Decode(nil, bufBytes)
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	return nil, nil
}

// tryBufferFromReader attempts to cast the reader to a `*bytes.Buffer` this is possible when using httpgrpc.
// If it fails it will return nil and false.
func tryBufferFromReader(reader io.Reader) (*bytes.Buffer, bool) {
	if bufReader, ok := reader.(interface {
		BytesBuffer() *bytes.Buffer
	}); ok && bufReader != nil {
		return bufReader.BytesBuffer(), true
	}
	return nil, false
}
