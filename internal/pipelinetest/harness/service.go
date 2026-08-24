package harness

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/remotecfg"
)

const (
	// httpListenAddr uses port 0 so the kernel picks a free port. Pipeline tests
	// run in parallel with other tests, so a fixed port would collide.
	httpListenAddr = "127.0.0.1:0"

	// memoryListenAddr matches the production default. It is an identifier, not a
	// real address: the HTTP service compares it by string to route a dial to its
	// in-memory listener. prometheus.exporter.* components publish targets on this
	// address, so a scrape only reaches them when both sides agree on the value.
	memoryListenAddr = "alloy.internal:12345"
)

func defaultServices(l *logging.Logger) []service.Service {
	return []service.Service{
		livedebugging.New(),
		labelstore.New(l.Slog(), prometheus.NewRegistry()),
		// The real HTTP service, not a mock. prometheus.exporter.* components serve
		// their metrics over the component HTTP path and export a target pointing at
		// the in-memory listener, so prometheus.scrape can only reach them when the
		// service actually serves and its DialFunc routes in-memory traffic.
		httpservice.New(httpservice.Options{
			Logger:           l,
			HTTPListenAddr:   httpListenAddr,
			MemoryListenAddr: memoryListenAddr,
			MinStability:     featuregate.StabilityExperimental,
			ReadyFunc:        func() bool { return true },
			ReloadFunc:       func() error { return nil },
		}),
		// The HTTP service declares remotecfg in DependsOn. Pipeline tests never hit
		// the remotecfg component path, so a stub is enough to satisfy the graph.
		&mockService{
			name: remotecfg.ServiceName,
		},
		&mockService{
			name: cluster.ServiceName,
			data: cluster.Mock(),
		},
	}
}

var _ service.Service = (*mockService)(nil)

type mockService struct {
	name string
	data any
}

func (s *mockService) Definition() service.Definition {
	return service.Definition{
		Name:       s.name,
		Stability:  featuregate.StabilityExperimental,
		DependsOn:  nil,
		ConfigType: nil,
	}
}

func (s *mockService) Run(ctx context.Context, host service.Host) error {
	<-ctx.Done()
	return nil
}

func (s *mockService) Update(newConfig any) error {
	return nil
}

func (s *mockService) Data() any {
	return s.data
}
