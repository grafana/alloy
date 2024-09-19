package types

import (
	"context"
)

type NetworkClient interface {
	Start()
	Stop()
	SendSeries(ctx context.Context, d *TimeSeriesBinary) error
	SendMetadata(ctx context.Context, d *TimeSeriesBinary) error
}
