package preview

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

type reportSample struct {
	labels   labels.Labels
	value    float64
	metadata metadata.Metadata
}

func (c *Component) snapshot(now time.Time) ([]reportSample, []storage.AppendableV2) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return nil, nil
	}
	var samples []reportSample
	add := func(name string, v float64, typ model.MetricType, ls labels.Labels) {
		builder := labels.NewBuilder(ls)
		builder.Set(model.MetricNameLabel, "prometheus_preview_"+name)
		samples = append(samples, reportSample{
			labels: builder.Labels(), value: v, metadata: metadata.Metadata{Type: typ},
		})
	}
	base := labels.FromStrings("preview_id", c.opts.ID)
	add("observation_start_time_seconds", float64(c.started.UnixMilli())/1000, model.MetricTypeGauge, base)
	add("window_start_time_seconds", float64(c.windowStarted.UnixMilli())/1000, model.MetricTypeGauge, base)
	add("window_duration_seconds", now.Sub(c.windowStarted).Seconds(), model.MetricTypeGauge, base)
	complete := now.Sub(c.windowStarted) >= c.args.Window
	add("window_complete", boolFloat(complete), model.MetricTypeGauge, base)

	addGroup := func(g *group, scope string) {
		builder := labels.NewBuilder(g.labels)
		builder.Set("preview_id", c.opts.ID)
		builder.Set("preview_scope", scope)
		ls := builder.Labels()
		add("series", float64(g.series), model.MetricTypeGauge, ls)
		add("series_limited", boolFloat(g.limited), model.MetricTypeGauge, ls)
		for kind, name := range []string{"float", "histogram"} {
			builder.Set("sample_type", name)
			add("samples_total", float64(g.samples[kind]), model.MetricTypeCounter, builder.Labels())
		}
	}
	addGroup(&c.stats.total, "total")
	if len(c.args.GroupBy) > 0 {
		for _, g := range c.stats.groups {
			addGroup(g, "group")
		}
		addGroup(&c.stats.overflow, "overflow")
	}
	slices.SortFunc(samples, func(a, b reportSample) int { return labels.Compare(a.labels, b.labels) })
	// Retain the exact generated report before rolling over the window. Local
	// inspection must not generate reports or advance measurement state.
	c.latestReport = &reportSnapshot{generatedAt: now, samples: samples}
	if complete {
		c.stats.resetWindow()
		c.windowStarted = now
	}
	return samples, c.args.ReportTo
}

func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (c *Component) report(ctx context.Context) error {
	now := c.now()
	samples, receivers := c.snapshot(now)
	var errs []error
	for i, receiver := range receivers {
		if err := sendReport(ctx, receiver, now.UnixMilli(), samples); err != nil {
			errs = append(errs, fmt.Errorf("report_to[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// Reporting never holds the aggregation lock. A failing destination cannot
// cause original samples to be forwarded, or make input transactions fail.
func sendReport(ctx context.Context, receiver storage.AppendableV2, timestamp int64, samples []reportSample) error {
	app := receiver.AppenderV2(ctx)
	for _, sample := range samples {
		_, err := app.Append(0, sample.labels, 0, timestamp, sample.value, nil, nil, storage.AppendV2Options{Metadata: sample.metadata})
		if err != nil {
			return errors.Join(err, app.Rollback())
		}
	}
	return app.Commit()
}
