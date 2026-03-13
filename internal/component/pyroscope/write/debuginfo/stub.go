//go:build !(linux && (arm64 || amd64))

package debuginfo

import (
	"context"

	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
)

type UploadJob struct {
	//no-op
}

func (c *Client) newUploader(j UploadJob) (*uploader, error) {
	return &uploader{}, nil
}

type uploader struct {
}

func (u uploader) upload(c debuginfov1alpha1connect.DebuginfoServiceClient, j UploadJob) {
	// no-op
}

func (u uploader) run(ctx context.Context) error {
	// no-op
	return nil
}
