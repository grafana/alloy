package testappender

import (
	"context"

	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/prometheus/appenders/adapter"
)

// AppenderVersion gets the appender of one Prometheus appender version from
// an appendable, as a storage.AppenderV2. This lets tests make the same
// appends through every version.
type AppenderVersion struct {
	Name     string
	Appender func(context.Context, storage.AppendableV2) storage.AppenderV2
}

// AppenderVersions returns the V1 appender, adapted with
// adapter.AppenderV1AsV2, and the V2 appender. The appendable must implement
// storage.Appendable as well, for the V1 appender.
//
// TODO: Remove the V1 appender when the V1 appenders are removed.
func AppenderVersions() []AppenderVersion {
	return []AppenderVersion{
		{
			Name: "V1",
			Appender: func(ctx context.Context, a storage.AppendableV2) storage.AppenderV2 {
				return adapter.AppenderV1AsV2(a.(storage.Appendable).Appender(ctx))
			},
		},
		{
			Name: "V2",
			Appender: func(ctx context.Context, a storage.AppendableV2) storage.AppenderV2 {
				return a.AppenderV2(ctx)
			},
		},
	}
}
