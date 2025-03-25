package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const (
	SetupConsumerName = "setup_consumer"
)

type SetupConsumerArguments struct {
	DSN      string
	Registry *prometheus.Registry
}

type setupConsumer struct {
	DSN        string
	Registry   *prometheus.Registry
	InfoMetric *prometheus.GaugeVec
	running    *atomic.Bool
}

func NewSetupConsumer(args SetupConsumerArguments) (*setupConsumer, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "setup_consumer_info",
		Help:      "Information about the setup consumer",
	}, []string{"status"})

	args.Registry.MustRegister(infoMetric)

	return &setupConsumer{
		DSN:        args.DSN,
		Registry:   args.Registry,
		InfoMetric: infoMetric,
		running:    &atomic.Bool{},
	}, nil
}

func (s *setupConsumer) Name() string {
	return SetupConsumerName
}

func (s *setupConsumer) Start(ctx context.Context) error {
	s.running.Store(true)
	s.InfoMetric.WithLabelValues("active").Set(1)
	return nil
}

func (s *setupConsumer) Stopped() bool {
	return !s.running.Load()
}

func (s *setupConsumer) Stop() {
	s.Registry.Unregister(s.InfoMetric)
	s.running.Store(false)
}
