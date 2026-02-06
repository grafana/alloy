package loghttp

// KEEP IN SYNC WITH:
// https://github.com/grafana/loki/blob/main/pkg/util/unmarshal/unmarshal.go
// Local modifications should be minimized.

import (
	"io"
	"unsafe"

	"github.com/grafana/loki/pkg/push"
	jsoniter "github.com/json-iterator/go"
)

// decodePushRequest directly decodes json to a push.PushRequest
func decodePushRequest(b io.Reader, r *push.PushRequest) error {
	var request PushRequest

	if err := jsoniter.NewDecoder(b).Decode(&request); err != nil {
		return err
	}

	*r = push.PushRequest{
		Streams: *(*[]push.Stream)(unsafe.Pointer(&request.Streams)), //#nosec G103 -- Just preventing an allocation, safe, there's no chance of an incorrect type cast here. -- nosemgrep: use-of-unsafe-block
	}

	return nil
}
