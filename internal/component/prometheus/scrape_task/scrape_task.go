package scrape_task

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
)

const (
	TaskTypeScrapeTaskV1 = "alloy:scrape_task:v1"
)

type ScrapeTask struct {
	Target discovery.Target
}

type ScrapeTaskProvider interface {
	Get() []ScrapeTask
}

type ScrapeTaskConsumer interface {
	Consume(tasks []ScrapeTask)
}

type MetricsConsumer interface {
	Consume(metrics []promadapter.Metrics)
}
