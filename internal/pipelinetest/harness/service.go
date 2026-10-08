package harness

import (
	"context"
	"fmt"
	"slices"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/features"
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

func defaultServices(l *logging.Logger, flags []string) ([]service.Service, error) {
	enabled, err := featuresFromFlags(flags)
	if err != nil {
		return nil, err
	}

	return []service.Service{
		features.New(enabled...),
		livedebugging.New(),
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

// featuresFromFlags returns the features named by flags, which use the CLI flag
// name without the leading dashes.
func featuresFromFlags(flags []string) ([]features.Feature, error) {
	enabled := make([]features.Feature, 0, len(flags))
	for _, flag := range flags {
		i := slices.IndexFunc(features.All, func(f features.Feature) bool { return f.Flag() == flag })
		if i < 0 {
			return nil, fmt.Errorf("unknown feature %q", flag)
		}
		enabled = append(enabled, features.All[i])
	}
	return enabled, nil
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
