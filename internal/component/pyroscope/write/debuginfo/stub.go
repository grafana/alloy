//go:build !(linux && (arm64 || amd64))

package debuginfo

import (
	"context"
)

type UploadJob struct {
	//no-op
}

func (c *Client) newUploader(j UploadJob) (*uploader, error) {
	return &uploader{}, nil
}

type uploader struct {
}

func (u uploader) upload(c DebugInfoClient, j UploadJob) {
	// no-op
}

func (u uploader) run(ctx context.Context) error {
	// no-op
	return nil
}
