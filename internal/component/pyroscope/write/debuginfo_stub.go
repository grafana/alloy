//go:build !(linux && (arm64 || amd64))

package write

import (
	"net/url"
)

func newDebugInfoUpload(u *url.URL, metrics *metrics, e *EndpointOptions) (debugInfoUploader, error) {
	return nil, nil
}
