package pyroscope

import (
	"context"

	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/prometheus/prometheus/model/labels"
)

var _ Appendable = AppenderMock{}

type AppenderMock struct {
	AppendIngestFunc      func(ctx context.Context, profile *IncomingProfile) error
	AppendFunc            func(ctx context.Context, labels labels.Labels, samples []*RawSample) error
	DebugInfoEndpointsFn  func() []debuginfo.Endpoint
	DebugInfoUploadFunc   func(j debuginfo.UploadJob)
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

func (a AppenderMock) DebugInfoEndpoints() []debuginfo.Endpoint {
	if a.DebugInfoEndpointsFn != nil {
		return a.DebugInfoEndpointsFn()
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
