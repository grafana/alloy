//go:build linux && (arm64 || amd64)

package ebpf

import (
	"context"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter"
)

const maxSendConcurrency = 32

func (c *Component) sendProfiles(ctx context.Context, ps []reporter.PPROF) {
	start := time.Now()
	pool := workerPool{}
	n := len(ps)
	pool.run(min(maxSendConcurrency, n))
	queued := 0
	ctx, cancel := context.WithTimeout(ctx, c.args.CollectInterval)
	defer func() {
		pool.stop()
		cancel()
		level.Debug(c.logger).Log("msg", "sent profiles", "duration", time.Since(start), "queued", queued)
	}()
	j := 0
	for _, p := range ps {
		serviceName := p.Labels.Get("service_name")
		c.metrics.pprofsTotal.WithLabelValues(serviceName).Inc()
		c.metrics.pprofSamplesTotal.WithLabelValues(serviceName).Add(float64(len(p.Raw)))

		rawProfile := p.Raw

		appender := c.appendable.Appender()
		c.metrics.pprofBytesTotal.WithLabelValues(serviceName).Add(float64(len(rawProfile)))

		job := func() {
			samples := []*pyroscope.RawSample{{RawProfile: rawProfile}}
			err := appender.Append(ctx, p.Labels, samples)
			if err != nil {
				level.Error(c.logger).Log("msg", "ebpf pprof write", "err", err)
			}
		}
		select {
		case pool.jobs <- job:
			queued++
		case <-ctx.Done():
			dropped := n - j
			c.metrics.pprofsDroppedTotal.Add(float64(dropped))
			level.Debug(c.logger).Log("msg", "dropped profiles", "count", dropped)
			return
		}
		j++
	}
}
