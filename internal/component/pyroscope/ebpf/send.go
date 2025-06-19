//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/pyroscope/ebpf/pprof"
)

const maxSendConcurrency = 32

func (c *Component) sendProfiles(ctx context.Context, builders *pprof.ProfileBuilders) {
	start := time.Now()
	pool := workerPool{}
	pool.run(min(maxSendConcurrency, len(builders.Builders)))
	queued := 0
	ctx, cancel := context.WithTimeout(ctx, c.args.CollectInterval)
	defer func() {
		pool.stop()
		cancel()
		level.Debug(c.options.Logger).Log("msg", "sent profiles", "duration", time.Since(start), "queued", queued)
	}()
	j := 0
	for _, builder := range builders.Builders {
		serviceName := builder.Labels.Get("service_name")
		c.metrics.pprofsTotal.WithLabelValues(serviceName).Inc()
		c.metrics.pprofSamplesTotal.WithLabelValues(serviceName).Add(float64(len(builder.Profile.Sample)))

		buf := bytes.NewBuffer(nil)
		_, err := builder.Write(buf)
		if err != nil {
			level.Error(c.options.Logger).Log("err", fmt.Errorf("ebpf profile encode %w", err))
			continue
		}
		rawProfile := buf.Bytes()

		appender := c.appendable.Appender()
		c.metrics.pprofBytesTotal.WithLabelValues(serviceName).Add(float64(len(rawProfile)))

		job := func() {
			samples := []*pyroscope.RawSample{{RawProfile: rawProfile}}
			err = appender.Append(ctx, builder.Labels, samples)
			if err != nil {
				level.Error(c.options.Logger).Log("msg", "ebpf pprof write", "err", err)
			}
		}
		select {
		case pool.jobs <- job:
			queued++
		case <-ctx.Done():
			dropped := len(builders.Builders) - j
			c.metrics.pprofsDroppedTotal.Add(float64(dropped))
			level.Debug(c.options.Logger).Log("msg", "dropped profiles", "count", dropped)
			return
		}
		j++
	}
}
