package scrape_task

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
)

type ScrapeTask struct {
	IssueTime time.Time
	Target    discovery.Target
}

type ScrapeTaskConsumer interface {
	Consume(tasks []ScrapeTask) map[int]error
}

type MetricsConsumer interface {
	Consume(metrics promadapter.Metrics) error
}
