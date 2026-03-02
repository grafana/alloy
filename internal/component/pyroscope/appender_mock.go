package pyroscope

import (
	"context"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/prometheus/prometheus/model/labels"
)

var _ Appendable = AppenderMock{}

type AppenderMock struct {
	AppendIngestFunc    func(ctx context.Context, profile *IncomingProfile) error
	AppendFunc          func(ctx context.Context, labels labels.Labels, samples []*RawSample) error
	ClientFunc          func() debuginfogrpc.DebuginfoServiceClient
	DebugInfoUploadFunc func(j debuginfo.UploadJob)
}

func (a AppenderMock) Append(ctx context.Context, labels labels.Labels, samples []*RawSample) error {
	return a.AppendFunc(ctx, labels, samples)
}

func (a AppenderMock) AppendIngest(ctx context.Context, profile *IncomingProfile) error {
	return a.AppendIngestFunc(ctx, profile)
}

func (a AppenderMock) Appender() Appender {
	return a
}

func (a AppenderMock) Client() debuginfogrpc.DebuginfoServiceClient {
	return a.ClientFunc()
}

func (a AppenderMock) Upload(j debuginfo.UploadJob) {
	a.DebugInfoUploadFunc(j)
}

func AppendableFunc(f func(ctx context.Context, labels labels.Labels, samples []*RawSample) error) AppenderMock {
	return AppenderMock{
		AppendFunc: f,
	}
}

func AppendableIngestFunc(f func(ctx context.Context, profile *IncomingProfile) error) AppenderMock {
	return AppenderMock{
		AppendIngestFunc: f,
	}
}
