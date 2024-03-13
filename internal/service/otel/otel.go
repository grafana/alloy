// Package otel implements the otel service for Flow.
// This service registers feature gates will be used by the otelcol components
// based on upstream Collector components.
package otel

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/service"
	"github.com/grafana/agent/internal/util"
)

// ServiceName defines the name used for the otel service.
const ServiceName = "otel"

type Service struct{}

var _ service.Service = (*Service)(nil)

func New(logger log.Logger) *Service {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	// The feature gates should be set in New() instead of Run().
	// Otel checks the feature gate very early, during the creation of
	// an Otel component. If we set the feature gates in Run(), it will
	// be too late - Otel would have already checked the feature gate by then.
	// This is because the services are not started prior to the graph evaluation.
	err := util.SetupFlowModeOtelFeatureGates()
	if err != nil {
		logger.Log("msg", "failed to set up Otel feature gates", "err", err)
		return nil
	}

	return &Service{}
}

// Data implements service.Service. It returns nil, as the otel service does
// not have any runtime data.
func (*Service) Data() any {
	return nil
}

// Definition implements service.Service.
func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: nil, // otel does not accept configuration
		DependsOn:  []string{},
		Stability:  featuregate.StabilityStable,
	}
}

// Run implements service.Service.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	<-ctx.Done()
	return nil
}

// Update implements service.Service.
func (*Service) Update(newConfig any) error {
	return fmt.Errorf("otel service does not support configuration")
}
