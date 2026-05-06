package harness

import (
	"context"
	"net"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/prometheus/client_golang/prometheus"
)

func defaultServices(l log.Logger) []service.Service {
	return []service.Service{
		livedebugging.New(),
		labelstore.New(l, prometheus.NewRegistry()),
		&mockService{
			name: httpservice.ServiceName,
			data: httpservice.Data{
				HTTPListenAddr:   "127.0.0.1:0",
				MemoryListenAddr: "alloy.internal:0",
				BaseHTTPPath:     "/",
				DialFunc:         (&net.Dialer{}).DialContext,
			},
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
