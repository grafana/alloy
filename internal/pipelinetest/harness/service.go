package harness

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	lokiservice "github.com/grafana/alloy/internal/service/loki"
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

func defaultServices(l *logging.Logger, flags []string) ([]service.Service, error) {
	lokiOpts, err := lokiOptionsFromFlags(flags)
	if err != nil {
		return nil, err
	}

	return []service.Service{
		livedebugging.New(),
		lokiservice.New(lokiOpts),
		labelstore.New(l.Slog(), prometheus.NewRegistry()),
		httpservice.New(httpservice.Options{
			Logger:           l,
			HTTPListenAddr:   httpListenAddr,
			MemoryListenAddr: memoryListenAddr,
			MinStability:     featuregate.StabilityExperimental,
			Gatherer:         prometheus.DefaultGatherer,
			ReadyFunc:        func() bool { return true },
			ReloadFunc:       func() error { return nil },
		}),
		// The HTTP service declares remotecfg in DependsOn. Pipeline tests do no
		// remote config management, so the no-op stub is enough.
		remotecfg.NewStub(prometheus.NewRegistry()),
		&mockService{
			name: cluster.ServiceName,
			data: cluster.Mock(),
		},
	}, nil
}

// lokiOptionsFromFlags returns the loki service options enabled by flags, which
// use the CLI flag name without the leading dashes.
func lokiOptionsFromFlags(flags []string) (lokiservice.Options, error) {
	var opts lokiservice.Options
	for _, flag := range flags {
		switch flag {
		case "feature.loki.consumer-pipeline.enabled":
			opts.ConsumerPipeline = true
		default:
			return lokiservice.Options{}, fmt.Errorf("unknown feature %q", flag)
		}
	}
	return opts, nil
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
