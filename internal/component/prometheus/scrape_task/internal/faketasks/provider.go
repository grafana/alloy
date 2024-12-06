package faketasks

import (
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promstub"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/random"
)

type provider struct {
	scrapeInterval         time.Duration
	tasksCount             int
	ticker                 *time.Ticker
	lagNotificationHandler func(duration time.Duration)

	mut     sync.Mutex
	cached  []scrape_task.ScrapeTask
	lastPop time.Time
}

func NewProvider(scrapeInterval time.Duration, tasksCount int, lagHandler func(duration time.Duration)) scrape_task.ScrapeTaskProvider {
	if lagHandler == nil {
		lagHandler = func(duration time.Duration) {}
	}
	return &provider{
		scrapeInterval:         scrapeInterval,
		tasksCount:             tasksCount,
		ticker:                 time.NewTicker(scrapeInterval),
		lagNotificationHandler: lagHandler,
		lastPop:                time.Now().Add(2 * scrapeInterval),
	}
}

func (p *provider) Get() []scrape_task.ScrapeTask {
	<-p.ticker.C // this limits the rate of scrape tasks produced, simulating scrape interval
	p.mut.Lock()
	defer p.mut.Unlock()
	p.lagNotificationHandler(time.Since(p.lastPop))
	p.lastPop = time.Now()
	if p.cached == nil {
		p.cached = fakeScrapeTasks(p.tasksCount)
	}
	return p.cached
}

func fakeScrapeTasks(count int) []scrape_task.ScrapeTask {
	result := make([]scrape_task.ScrapeTask, count)
	for i := 0; i < count; i++ {
		numberOfSeries := random.NumberOfSeries(1_000, 100_000, 1_000)
		result[i] = scrape_task.ScrapeTask{
			Target: discovery.Target{
				"host":                         "host_" + random.String(6),
				"team":                         "team_" + random.String(1),
				promstub.SeriesToGenerateLabel: numberOfSeries,
			},
		}
	}
	return result
}
