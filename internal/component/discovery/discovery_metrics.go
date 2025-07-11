package discovery

import (
	"context"
	"log/slog"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	promdiscovery "github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/grafana/alloy/internal/runtime/logging"
)

type DiscovererWithMetrics interface {
	promdiscovery.Discoverer
	promdiscovery.DiscovererMetrics
}

type discovererWithMetrics struct {
	discoverer     promdiscovery.Discoverer
	refreshMetrics promdiscovery.DiscovererMetrics
	sdMetrics      promdiscovery.DiscovererMetrics
}

func NewDiscovererWithMetrics(cfg promdiscovery.Config, reg prometheus.Registerer, logger log.Logger) (DiscovererWithMetrics, error) {
	refreshMetrics := promdiscovery.NewRefreshMetrics(reg)
	cfg.NewDiscovererMetrics(reg, refreshMetrics)

	sdMetrics := cfg.NewDiscovererMetrics(reg, refreshMetrics)

	discoverer, err := cfg.NewDiscoverer(promdiscovery.DiscovererOptions{
		Logger:  slog.New(logging.NewSlogGoKitHandler(logger)),
		Metrics: sdMetrics,
	})

	if err != nil {
		return nil, err
	}

	return &discovererWithMetrics{
		discoverer:     discoverer,
		refreshMetrics: refreshMetrics,
		sdMetrics:      sdMetrics,
	}, nil
}

func (d *discovererWithMetrics) Run(ctx context.Context, up chan<- []*targetgroup.Group) {
	d.discoverer.Run(ctx, up)
}

func (d *discovererWithMetrics) Register() error {
	if err := d.refreshMetrics.Register(); err != nil {
		return err
	}
	return d.sdMetrics.Register()
}

func (d *discovererWithMetrics) Unregister() {
	d.refreshMetrics.Unregister()
	d.sdMetrics.Unregister()
}

var _ DiscovererWithMetrics = (*discovererWithMetrics)(nil)
