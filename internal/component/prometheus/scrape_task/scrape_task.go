package scrape_task

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
)

type ScrapeTask struct {
	Target discovery.Target
}

type ScrapeTaskConsumer interface {
	Consume(tasks []ScrapeTask)
}

type MetricsConsumer interface {
	Consume(metrics []promadapter.Metrics)
}
