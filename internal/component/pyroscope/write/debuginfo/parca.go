//go:build linux && (arm64 || amd64)

package debuginfo

import (
	"context"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter/parca/reporter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
)

type UploadJob struct {
	FrameMappingFileData libpf.FrameMappingFileData
	Open                 func() (process.ReadAtCloser, error)
	// InitArguments is the structure used to create a new parca uploader
	// it is passed as the job field to have the configuration in the ebpf component instead of write component,
	// to not confuse users.
	InitArguments Arguments
}

func (c *Client) newUploader(j UploadJob) (*uploader, error) {
	args := j.InitArguments
	u, err := reporter.NewParcaSymbolUploader(
		c.logger,
		args.CacheSize,
		args.StripTextSection,
		args.QueueSize,
		args.WorkerNum,
		c.dataPath,
		c.metric,
	)
	if err != nil {
		return nil, err
	}
	return &uploader{u: u}, nil
}

type uploader struct {
	u *reporter.ParcaSymbolUploader
}

func (u *uploader) upload(c debuginfogrpc.DebuginfoServiceClient, j UploadJob) {
	u.u.Upload(context.Background(),
		c,
		j.FrameMappingFileData.FileID,
		j.FrameMappingFileData.FileName.String(),
		j.FrameMappingFileData.GnuBuildID,
		j.Open,
	)
}

func (u *uploader) run(ctx context.Context) error {
	return u.u.Run(ctx)
}
