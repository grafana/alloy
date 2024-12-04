package promstub

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
)

func NewSender() promadapter.Sender {
	return &sender{}
}

type sender struct{}

func (s *sender) Send(metrics []promadapter.Metrics) error {
	// Marshal the messages to simulate some work
	for _, m := range metrics {
		for _, ts := range m.TimeSeries {
			b, err := ts.Marshal()
			if err != nil || len(b) == 0 {
				return err
			}
		}
	}

	simulateLatency(
		time.Microsecond*500, // min
		time.Millisecond*300, // avg
		time.Second*10,       // max
		time.Second,          // stdev
	)

	// 1% failures
	if rand.Intn(100) == 0 {
		return fmt.Errorf("send failed: test downstream unavailable")
	}

	return nil
}

func simulateLatency(minLatency time.Duration, avgLatency time.Duration, maxLatency time.Duration, stdDev time.Duration) {
	thisRequestLatency := time.Duration(rand.NormFloat64()*float64(stdDev) + float64(avgLatency))
	if thisRequestLatency < minLatency {
		thisRequestLatency = minLatency
	}
	if thisRequestLatency > maxLatency {
		thisRequestLatency = maxLatency
	}

	time.Sleep(thisRequestLatency)
}
