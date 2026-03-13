package pyroscope

import (
	"context"

	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/prometheus/model/labels"
)

var _ Appendable = AppenderMock{}

type AppenderMock struct {
	AppendIngestFunc    func(ctx context.Context, profile *IncomingProfile) error
	AppendFunc          func(ctx context.Context, labels labels.Labels, samples []*RawSample) error
	ConnectClientFunc   func() debuginfov1alpha1connect.DebuginfoServiceClient
	ConnectClientsFunc  func() []debuginfov1alpha1connect.DebuginfoServiceClient
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

func (a AppenderMock) ConnectClient() debuginfov1alpha1connect.DebuginfoServiceClient {
	if a.ConnectClientFunc != nil {
		return a.ConnectClientFunc()
	}
	return nil
}

func (a AppenderMock) ConnectClients() []debuginfov1alpha1connect.DebuginfoServiceClient {
	if a.ConnectClientsFunc != nil {
		return a.ConnectClientsFunc()
	}
	return nil
}

func (a AppenderMock) Upload(j debuginfo.UploadJob) {
	if a.DebugInfoUploadFunc != nil {
		a.DebugInfoUploadFunc(j)
	}
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
