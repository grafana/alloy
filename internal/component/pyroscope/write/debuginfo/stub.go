//go:build !(linux && (arm64 || amd64))

package debuginfo

import (
	"context"

	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfoclient"
)

type UploadJob struct {
	//no-op
}

func (c *Uploader) newUploader(j UploadJob) (*uploader, error) {
	return &uploader{}, nil
}

type uploader struct {
}

func (u uploader) upload(c *debuginfoclient.Client, j UploadJob) {
	// no-op
}

func (u uploader) run(ctx context.Context) error {
	// no-op
	return nil
}
