package pyroscope

import (
	"context"

	"github.com/prometheus/prometheus/model/labels"
)

type AppenderMock struct {
	AppendIngestFunc func(ctx context.Context, profile *IncomingProfile) error
	AppendFunc       func(ctx context.Context, labels labels.Labels, samples []*RawSample) error
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
