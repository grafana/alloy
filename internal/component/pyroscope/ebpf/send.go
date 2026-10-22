//go:build linux && (arm64 || amd64)

package ebpf

import (
	"context"
	"time"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter"
)

const maxSendConcurrency = 32

func (c *Component) sendProfiles(ctx context.Context, ps []reporter.PPROF) {
	c.argsMut.RLock()
	interval, batchEnabled := c.args.CollectInterval, c.args.BatchEnabled
	c.argsMut.RUnlock()
	if batchEnabled {
		c.sendProfilesBatch(ctx, ps, interval)
		return
	}
	start := time.Now()
	pool := workerPool{}
	n := len(ps)
	pool.run(min(maxSendConcurrency, n))
	queued := 0
	ctx, cancel := context.WithTimeout(ctx, interval)
	defer func() {
		pool.stop()
		cancel()
		c.logger.Debug("sent profiles", "duration", time.Since(start), "queued", queued)
	}()
	j := 0
	for _, p := range ps {
		serviceName := p.Labels.Get("service_name")
		c.metrics.pprofsTotal.WithLabelValues(serviceName).Inc()
		c.metrics.pprofSamplesTotal.WithLabelValues(serviceName).Add(float64(p.Samples))

		rawProfile := p.Raw

		appender := c.appendable.Appender()
		c.metrics.pprofBytesTotal.WithLabelValues(serviceName).Add(float64(len(rawProfile)))

		job := func() {
			samples := []*pyroscope.RawSample{{RawProfile: rawProfile}}
			err := appender.Append(ctx, p.Labels, samples)
			if err != nil {
				c.logger.Error("ebpf pprof write", "err", err)
			}
		}
		select {
		case pool.jobs <- job:
			queued++
		case <-ctx.Done():
			dropped := n - j
			c.metrics.pprofsDroppedTotal.Add(float64(dropped))
			c.logger.Debug("dropped profiles", "count", dropped)
			return
		}
		j++
	}
}

func (c *Component) sendProfilesBatch(ctx context.Context, ps []reporter.PPROF, interval time.Duration) {
	if len(ps) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, interval)
	defer cancel()

	// Allocate metadata in contiguous slices; reuse the already encoded pprof bytes.
	series := make([]pyroscope.RawProfileSeries, len(ps))
	samples := make([]pyroscope.RawSample, len(ps))
	pointers := make([]*pyroscope.RawSample, len(ps))
	for i, p := range ps {
		if ctx.Err() != nil {
			c.metrics.pprofsDroppedTotal.Add(float64(len(ps)))
			return
		}
		serviceName := p.Labels.Get("service_name")
		c.metrics.pprofsTotal.WithLabelValues(serviceName).Inc()
		c.metrics.pprofSamplesTotal.WithLabelValues(serviceName).Add(float64(p.Samples))
		c.metrics.pprofBytesTotal.WithLabelValues(serviceName).Add(float64(len(p.Raw)))
		samples[i].RawProfile = p.Raw
		pointers[i] = &samples[i]
		series[i] = pyroscope.RawProfileSeries{Labels: p.Labels, Samples: pointers[i : i+1 : i+1]}
	}
	if err := pyroscope.AppendBatch(ctx, c.appendable.Appender(), series); err != nil {
		c.logger.Error("ebpf pprof batch write", "err", err)
	}
}
