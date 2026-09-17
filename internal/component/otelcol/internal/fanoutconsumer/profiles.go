package fanoutconsumer

import (
	"context"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/multierr"
)

// Profiles creates a new fanout consumer for profiles.
func Profiles(in []otelcol.Consumer) xconsumer.Profiles {
	var consumers []xconsumer.Profiles
	for _, consumer := range in {
		if consumer == nil {
			continue
		}
		consumers = append(consumers, consumer)
	}

	if len(consumers) == 0 {
		return &profilesFanout{}
	} else if len(consumers) == 1 {
		return consumers[0]
	}

	fanout := &profilesFanout{}
	for _, consumer := range consumers {
		if consumer.Capabilities().MutatesData {
			fanout.mutable = append(fanout.mutable, consumer)
		} else {
			fanout.readonly = append(fanout.readonly, consumer)
		}
	}
	return fanout
}

type profilesFanout struct {
	mutable  []xconsumer.Profiles
	readonly []xconsumer.Profiles
}

func (f *profilesFanout) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: len(f.mutable) > 0 && len(f.readonly) == 0}
}

func (f *profilesFanout) ConsumeProfiles(ctx context.Context, pd pprofile.Profiles) error {
	var errs error

	if len(f.mutable) > 0 {
		for i := 0; i < len(f.mutable)-1; i++ {
			errs = multierr.Append(errs, f.mutable[i].ConsumeProfiles(ctx, cloneProfiles(pd)))
		}

		last := f.mutable[len(f.mutable)-1]
		if len(f.readonly) == 0 && !pd.IsReadOnly() {
			errs = multierr.Append(errs, last.ConsumeProfiles(ctx, pd))
		} else {
			errs = multierr.Append(errs, last.ConsumeProfiles(ctx, cloneProfiles(pd)))
		}
	}

	if len(f.readonly) > 1 && !pd.IsReadOnly() {
		pd.MarkReadOnly()
	}
	for _, consumer := range f.readonly {
		errs = multierr.Append(errs, consumer.ConsumeProfiles(ctx, pd))
	}

	return errs
}

func cloneProfiles(pd pprofile.Profiles) pprofile.Profiles {
	clonedProfiles := pprofile.NewProfiles()
	pd.CopyTo(clonedProfiles)
	return clonedProfiles
}
