//go:build !(linux && (arm64 || amd64))

package debuginfo

import (
	"context"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
)

type UploadJob struct {
	//no-op
}

func (c *Client) newUploader(j UploadJob) (*uploader, error) {
	return &uploader{}, nil
}

type uploader struct {
}

func (u uploader) upload(c debuginfogrpc.DebuginfoServiceClient, j UploadJob) {
	// no-op
}

func (u uploader) run(ctx context.Context) error {
	// no-op
	return nil
}
