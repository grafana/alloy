package pyroscope

import (
	"context"

	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
)

type AppenderMock struct {
	AppendIngestFunc    func(ctx context.Context, profile *IncomingProfile) error
	AppendFunc          func(ctx context.Context, labels labels.Labels, samples []*RawSample) error
	UploadDebugInfoFunc func(ctx context.Context, fileID libpf.FileID, fileName string, buildID string, open func() (process.ReadAtCloser, error))
}

func (a AppenderMock) Append(ctx context.Context, labels labels.Labels, samples []*RawSample) error {
	return a.AppendFunc(ctx, labels, samples)
}

func (a AppenderMock) AppendIngest(ctx context.Context, profile *IncomingProfile) error {
	return a.AppendIngestFunc(ctx, profile)
}

func (a AppenderMock) UploadDebugInfo(ctx context.Context, fileID libpf.FileID, fileName string, buildID string, open func() (process.ReadAtCloser, error)) {
	a.UploadDebugInfoFunc(ctx, fileID, fileName, buildID, open)
}

func (a AppenderMock) Appender() Appender {
	return a
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
