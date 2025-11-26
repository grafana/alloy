package loghttp

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/grafana/loki/pkg/push"

	"github.com/grafana/alloy/internal/loki/util"
)

var (
	contentType = http.CanonicalHeaderKey("Content-Type")
	contentEnc  = http.CanonicalHeaderKey("Content-Encoding")
)

const applicationJSON = "application/json"

// ParsePushRequest returns push.PushRequest from http.Request body, deserialized according to specified content type.
func ParsePushRequest(r *http.Request, maxRecvMsgSize int) (*push.PushRequest, error) {
	// Body
	var body io.Reader
	contentEncoding := r.Header.Get(contentEnc)
	switch contentEncoding {
	case "", "snappy":
		body = r.Body
	case "gzip":
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		body = gzipReader
	case "deflate":
		flateReader := flate.NewReader(r.Body)
		defer flateReader.Close()
		body = flateReader
	default:
		return nil, fmt.Errorf("Content-Encoding %q not supported", contentEncoding)
	}

	contentType := r.Header.Get(contentType)
	contentType, _ /* params */, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}

	var req push.PushRequest
	switch contentType {
	case applicationJSON:
		if err = decodePushRequest(body, &req); err != nil {
			return nil, err
		}
	default:
		// When no content-type header is set or when it is set to
		// `application/x-protobuf`: expect snappy compression.
		if err := util.ParseProtoReader(r.Context(), body, int(r.ContentLength), maxRecvMsgSize, &req, util.RawSnappy); err != nil {
			return nil, err
		}
	}

	return &req, nil
}
