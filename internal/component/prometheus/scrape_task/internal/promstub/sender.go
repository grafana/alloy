package promstub

import "github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"

func NewSender() promadapter.Sender {
	return &sender{}
}

type sender struct{}

func (s *sender) Send(metrics []promadapter.Metrics) error {
	// TODO(thampiotr): have some random write latency and some level of errors happening when sending here for demo
	return nil
}
