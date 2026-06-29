package github_exporter

import (
	"context"
	"log/slog"

	"github.com/google/go-github/v76/github"
	"github.com/prometheus/client_golang/prometheus"
)

type rateLimitCollector struct {
	log       *slog.Logger
	client    *github.Client
	limit     *prometheus.Desc
	remaining *prometheus.Desc
	reset     *prometheus.Desc
}

func newRateLimitCollector(log *slog.Logger, client *github.Client) *rateLimitCollector {
	return &rateLimitCollector{
		log:    log,
		client: client,
		limit: prometheus.NewDesc(
			prometheus.BuildFQName("github", "rate", "limit"),
			"Number of API queries allowed in a 60 minute window",
			[]string{},
			nil,
		),
		remaining: prometheus.NewDesc(
			prometheus.BuildFQName("github", "rate", "remaining"),
			"Number of API queries remaining in the current window",
			[]string{},
			nil,
		),
		reset: prometheus.NewDesc(
			prometheus.BuildFQName("github", "rate", "reset"),
			"The time at which the current rate limit window resets in UTC epoch seconds",
			[]string{},
			nil,
		),
	}
}

func (c *rateLimitCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.limit
	ch <- c.remaining
	ch <- c.reset
}

func (c *rateLimitCollector) Collect(ch chan<- prometheus.Metric) {
	rates, _, err := c.client.RateLimit.Get(context.Background())
	if err != nil {
		c.log.Error("error gathering GitHub rates", "err", err)
		return
	}
	if rates == nil || rates.Core == nil {
		c.log.Debug("GitHub core rate limit is unavailable")
		return
	}

	ch <- prometheus.MustNewConstMetric(c.limit, prometheus.GaugeValue, float64(rates.Core.Limit))
	ch <- prometheus.MustNewConstMetric(c.remaining, prometheus.GaugeValue, float64(rates.Core.Remaining))
	ch <- prometheus.MustNewConstMetric(c.reset, prometheus.GaugeValue, float64(rates.Core.Reset.Unix()))
}
