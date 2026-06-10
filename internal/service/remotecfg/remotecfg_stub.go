package remotecfg

import (
	"context"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/syntax/ast"
)

var _ service.Service = (*ServiceStub)(nil)

// ServiceStub is a no-op implementation of remote config service.
//
// Used instead of [Service] in OTel extension mode, where OpAMP already handles config management.
// The stub doesn't do config management but still provides a minimal implementation needed for support bundles and other services.
type ServiceStub struct {
	mut         sync.RWMutex
	systemAttrs map[string]string
	metrics     *metrics
	ctrl        service.Controller
}

// NewStub returns a new remote config service stub.
func NewStub(reg prometheus.Registerer) *ServiceStub {
	metrics := registerMetrics(reg)

	return &ServiceStub{
		systemAttrs: getSystemAttributes(),
		metrics:     metrics,
	}
}

func (s *ServiceStub) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: Arguments{},
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

func (s *ServiceStub) Run(ctx context.Context, host service.Host) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	c, err := host.NewController(ServiceName)
	if err != nil {
		return fmt.Errorf("failed to create controller for %s: %w", ServiceName, err)
	}

	s.ctrl = c
	return nil
}

func (s *ServiceStub) Update(_ any) error {
	// No-op: Update is called even if remotecfg block is not present.
	return nil
}

func (s *ServiceStub) Data() any {
	s.mut.RLock()
	defer s.mut.RUnlock()

	if s.ctrl == nil {
		return Data{Host: nil}
	}

	host := s.ctrl.(alloy_runtime.ServiceController).GetHost()
	return Data{Host: host}
}

func (s *ServiceStub) GetCachedAstFile() *ast.File {
	// Nothing to return as no config is managed.
	return nil
}
