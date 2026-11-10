package preview

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util/testappender"
)

func testArguments() Arguments {
	a := DefaultArguments
	a.ReportTo = []storage.AppendableV2{appenders.Noop{}}
	return a
}

func newTestComponent(t *testing.T, args Arguments) *Component {
	t.Helper()
	var exported component.Exports
	c, err := New(component.Options{
		ID:            "prometheus.preview.candidate",
		Logger:        logging.NewSlogNop(),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) { exported = e },
	}, args)
	require.NoError(t, err)
	require.Same(t, c, exported.(Exports).Receiver)
	return c
}

func TestArguments(t *testing.T) {
	var defaults Arguments
	defaults.SetToDefault()
	require.Equal(t, DefaultArguments, defaults)

	for _, tc := range []struct {
		name   string
		change func(*Arguments)
		err    string
	}{
		{"defaults", func(*Arguments) {}, ""},
		{"no reports", func(a *Arguments) { a.ReportTo = nil }, "report_to"},
		{"nil receiver", func(a *Arguments) { a.ReportTo = []storage.AppendableV2{nil} }, "nil receivers"},
		{"zero interval", func(a *Arguments) { a.ReportInterval = 0 }, "report_interval"},
		{"negative interval", func(a *Arguments) { a.ReportInterval = -time.Second }, "report_interval"},
		{"short window", func(a *Arguments) { a.Window = time.Second }, "window"},
		{"zero groups", func(a *Arguments) { a.MaxGroups = 0 }, "max_groups"},
		{"negative series", func(a *Arguments) { a.MaxSeries = -1 }, "max_series"},
		{"duplicate group", func(a *Arguments) { a.GroupBy = []string{"job", "job"} }, "duplicate"},
		{"invalid group", func(a *Arguments) { a.GroupBy = []string{"a-b"} }, "invalid"},
		{"long group", func(a *Arguments) { a.GroupBy = []string{strings.Repeat("a", 129)} }, "invalid"},
		{"many groups", func(a *Arguments) { a.GroupBy = make([]string, 17) }, "at most 16"},
		{"metric name", func(a *Arguments) { a.GroupBy = []string{"__name__", "namespace"} }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testArguments()
			tc.change(&a)
			err := a.Validate()
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func appendSample(t *testing.T, c *Component, name string) {
	t.Helper()
	app := c.Appender(t.Context())
	_, err := app.Append(0, labels.FromStrings("__name__", name), 1, 42)
	require.NoError(t, err)
	require.NoError(t, app.Commit())
}

func TestUpdate(t *testing.T) {
	args := testArguments()
	c := newTestComponent(t, args)
	appendSample(t, c, "first")
	originalStart := c.started

	// Re-evaluation, destination changes, and interval changes preserve data.
	require.NoError(t, c.Update(args))
	args.ReportTo = []storage.AppendableV2{&appenders.Noop{}}
	args.ReportInterval = 2 * time.Minute
	require.NoError(t, c.Update(args))
	require.Equal(t, uint64(1), c.stats.total.series)
	require.Equal(t, originalStart, c.started)

	invalid := args
	invalid.MaxSeries = 0
	require.Error(t, c.Update(invalid))
	require.Equal(t, args, c.args)

	app := c.Appender(t.Context())
	_, err := app.Append(0, labels.FromStrings("__name__", "old_generation"), 1, 1)
	require.NoError(t, err)
	c.now = func() time.Time { return originalStart.Add(time.Hour) }
	args.GroupBy = []string{"job"}
	require.NoError(t, c.Update(args))
	require.Empty(t, c.stats.series)
	require.Equal(t, [2]uint64{}, c.stats.total.samples)
	require.Equal(t, originalStart.Add(time.Hour), c.started)
	require.ErrorContains(t, app.Commit(), "configuration changed")
}

func TestRunReportsAndStops(t *testing.T) {
	sink := testappender.NewCollectingAppender()
	args := testArguments()
	args.ReportTo = []storage.AppendableV2{testappender.ConstantAppendable{Inner: sink}}
	args.ReportInterval = 5 * time.Millisecond
	c := newTestComponent(t, args)
	appendSample(t, c, "secret_metric")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	require.Eventually(t, func() bool { return len(sink.CollectedSamples()) > 0 }, time.Second, time.Millisecond)
	cancel()
	require.NoError(t, <-done)
	app := c.Appender(t.Context())
	require.ErrorContains(t, app.Commit(), "stopped")
	require.Nil(t, c.stats)
}

func TestConcurrentInputReportingAndUpdate(t *testing.T) {
	args := testArguments()
	c := newTestComponent(t, args)
	var wg sync.WaitGroup
	for worker := range 4 {
		wg.Go(func() {
			for i := range 50 {
				app := c.Appender(t.Context())
				_, err := app.Append(0, labels.FromStrings("__name__", fmt.Sprintf("metric_%d_%d", worker, i)), 1, 1)
				if err != nil {
					t.Error(err)
				}
				if err := app.Commit(); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Go(func() {
		for range 50 {
			if err := c.report(t.Context()); err != nil {
				t.Error(err)
			}
			if err := c.Update(args); err != nil {
				t.Error(err)
			}
		}
	})
	wg.Go(func() {
		for range 50 {
			_ = c.DebugInfo()
		}
	})
	wg.Wait()
	require.Equal(t, uint64(200), c.stats.total.series)
	require.Equal(t, uint64(200), c.stats.total.samples[0])
}
