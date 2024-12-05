package queuestub

import (
	"fmt"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promstub"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/random"
)

// The scrape interval we simulate here - how often scrape tasks are available in the queue.
const scrapeInterval = 3 * time.Second

// We only generate tasks once and keep them cached. To simulate same targets having same numbers of series.
var (
	cached  []scrape_task.ScrapeTask = nil
	mut     sync.Mutex
	ticker  = time.NewTicker(scrapeInterval)
	lastPop = time.Now().Add(scrapeInterval * 2)
)

func PopTasks(count int) []scrape_task.ScrapeTask {
	<-ticker.C // this limits the rate of scrape tasks produced, simulating scrape interval

	// TODO(thampiotr): Instead of this, in a real system we would monitor the queue depth. Kinda like WAL delay - a
	//                  sign of congestion.
	if time.Since(lastPop) > scrapeInterval*2 {
		fmt.Println("=======> QUEUE IS NOT DRAINED FAST ENOUGH")
	}
	lastPop = time.Now()

	return fakeScrapeTasks(count)
}

func fakeScrapeTasks(count int) []scrape_task.ScrapeTask {
	mut.Lock()
	defer mut.Unlock()
	if cached != nil {
		return cached
	}

	result := make([]scrape_task.ScrapeTask, count)
	for i := 0; i < count; i++ {
		numberOfSeries := random.NumberOfSeries(5_000, 100_000, 1_000)
		result[i] = scrape_task.ScrapeTask{
			Target: discovery.Target{
				"host":                         "host_" + random.String(6),
				"team":                         "team_" + random.String(1),
				promstub.SeriesToGenerateLabel: numberOfSeries,
			},
		}
	}

	cached = result
	return result
}
