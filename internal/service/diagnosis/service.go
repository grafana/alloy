package diagnosis

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	"github.com/prometheus/client_golang/prometheus"
)

// ServiceName defines the name used for the diagnosis service.
const ServiceName = "diagnosis"

type Options struct {
	Log     log.Logger
	Metrics prometheus.Registerer
}

type Service struct {
	diagnosis *diagnosis
}

var _ service.Service = (*Service)(nil)

func New(opts Options) *Service {
	return &Service{
		diagnosis: newDiagnosis(opts.Log, opts.Metrics),
	}
}

type Arguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Enabled: true,
	}
}

// Data implements service.Service.
// It returns the diagnosis for the components to stream.
func (s *Service) Data() any {
	return s
}

// Definition implements service.Service.
func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: Arguments{},
		DependsOn:  []string{},
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

// Run implements service.Service.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	return s.diagnosis.run(ctx, host)
}

// Update implements service.Service.
func (s *Service) Update(args any) error {
	newArgs := args.(Arguments)
	fmt.Println("diagnosis enabled", newArgs.Enabled)
	s.diagnosis.SetEnabled(newArgs.Enabled)
	return nil
}
