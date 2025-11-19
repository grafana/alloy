package reporter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
)

func TestCollectorReporterReportTraceEvent(t *testing.T) {
	for _, tt := range []struct {
		name   string
		trace  *libpf.Trace
		meta   *samples.TraceEventMeta
		nextFn xconsumer.ConsumeProfilesFunc
	}{
		{
			name:  "with no next consumer",
			trace: &libpf.Trace{},
			meta:  &samples.TraceEventMeta{},
		},
		{
			name:  "with a next consumer that succeeds",
			trace: &libpf.Trace{},
			meta:  &samples.TraceEventMeta{},
			nextFn: func(_ context.Context, _ pprofile.Profiles) error {
				return nil
			},
		},
		{
			name:  "with a next consumer that returns an error",
			trace: &libpf.Trace{},
			meta:  &samples.TraceEventMeta{},
			nextFn: func(_ context.Context, _ pprofile.Profiles) error {
				return errors.New("next consumer failed")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var next xconsumer.Profiles

			if tt.nextFn != nil {
				var err error
				next, err = xconsumer.NewProfiles(tt.nextFn)
				require.NoError(t, err)
			}

			r, err := NewCollector(&Config{}, next)
			require.NoError(t, err)
			if err := r.ReportTraceEvent(tt.trace, tt.meta); err != nil &&
				!errors.Is(err, errUnknownOrigin) {
				t.Fatal(err)
			}
		})
	}
}
