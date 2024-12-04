package queuestub

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promstub"
)

func PopTasks(maxTasks int) []scrape_task.ScrapeTask {
	return fakeScrapeTasks(maxTasks, 5000)
}

func fakeScrapeTasks(maxTasks int, maxSeriesPerTarget int) []scrape_task.ScrapeTask {
	tasks := rand.Intn(maxTasks)
	result := make([]scrape_task.ScrapeTask, tasks)
	for i := 0; i < tasks; i++ {
		result[i] = scrape_task.ScrapeTask{
			IssueTime: time.Now(),
			Target: discovery.Target{
				"host":                         "host_" + randomString(6),
				"team":                         "team_" + randomString(1),
				promstub.SeriesToGenerateLabel: strconv.Itoa(rand.Intn(maxSeriesPerTarget)),
			},
		}
	}
	return result
}

func randomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
