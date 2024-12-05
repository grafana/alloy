package promstub

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/random"
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
			// Each series adds latency, so the more series, the more latency.
			random.SimulateLatency(
				time.Nanosecond*10,   // min
				time.Nanosecond*100,  // avg
				time.Microsecond*100, // max
				time.Nanosecond*500,  // stdev
			)
		}
	}

	// 1% failures
	if rand.Intn(100) == 0 {
		return fmt.Errorf("send failed: test downstream unavailable")
	}

	return nil
}
