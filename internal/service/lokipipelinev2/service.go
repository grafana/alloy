// Package lokipipelinev2 implements the lokipipelinev2 service, which exposes the
// --feature.loki.pipelineV2.enabled flag to components. The service is
// temporary and will be removed once the migration of Loki components to
// pipeline V2 is complete (https://github.com/grafana/alloy/issues/4953).
package lokipipelinev2

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
)

// ServiceName defines the name used for the lokipipelinev2 service.
const ServiceName = "lokipipelinev2"

// Options configures the lokipipelinev2 service.
type Options struct {
	// Enabled enables passing logs between Loki components through
	// consumers instead of channels.
	Enabled bool
}

// Data is the runtime data exposed by the lokipipelinev2 service.
type Data interface {
	// Enabled reports whether Loki pipeline V2 is enabled.
	Enabled() bool
}

type Service struct {
	opts Options
}

var (
	_ service.Service = (*Service)(nil)
	_ Data            = (*Service)(nil)
)

// New creates a new lokipipelinev2 service.
func New(opts Options) *Service {
	return &Service{opts: opts}
}

func (*Service) Run(ctx context.Context, _ service.Host) error {
	<-ctx.Done()
	return nil
}

func (*Service) Update(_ any) error {
	return fmt.Errorf("lokipipelinev2 service does not support configuration")
}

func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: nil,
		DependsOn:  []string{},
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

func (s *Service) Data() any {
	return s
}

func (s *Service) Enabled() bool {
	return s.opts.Enabled
}
