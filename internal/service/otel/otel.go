// Package otel implements the otel service.
// This service registers feature gates will be used by the otelcol components
// based on upstream Collector components.
package otel

import (
	"context"
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/util"
)

// ServiceName defines the name used for the otel service.
const ServiceName = "otel"

type Service struct{}

var _ service.Service = (*Service)(nil)

func New(logger log.Logger) *Service {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	// Exemplars are enabled by default with https://github.com/open-telemetry/opentelemetry-go/releases/tag/v1.31.0
	// There is an error when converting the internal metrics to Prometheus format to expose them to the /metrics endpoint
	// "exemplar label name "net.host.port" is invalid"
	// The only way to disable the exemplars for now is to use an env variable.
	// The ability to disable it through the code has been added but not yet released.
	// This only affects the internal metrics of Alloy, not metrics that are sent to Alloy.
	// TODO: Add exemplar support and remove this env var
	_ = os.Setenv("OTEL_METRICS_EXEMPLAR_FILTER", "always_off")

	// The feature gates should be set in New() instead of Run().
	// Otel checks the feature gate very early, during the creation of
	// an Otel component. If we set the feature gates in Run(), it will
	// be too late - Otel would have already checked the feature gate by then.
	// This is because the services are not started prior to the graph evaluation.
	err := util.SetupOtelFeatureGates()
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
		Stability:  featuregate.StabilityGenerallyAvailable,
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
