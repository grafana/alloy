// Package lokipipeline implements the lokipipeline service, which exposes the
// --feature.loki.pipeline-v2.enabled flag to components. The service is
// temporary and will be removed once the migration of Loki components to
// pipeline v2 is complete (https://github.com/grafana/alloy/issues/4953).
package lokipipeline

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
)

// ServiceName defines the name used for the lokipipeline service.
const ServiceName = "lokipipeline"

// Options configures the lokipipeline service.
type Options struct {
	// PipelineV2 enables passing logs between Loki components through
	// consumers instead of channels.
	PipelineV2 bool
}

// Data is the runtime data exposed by the lokipipeline service.
type Data interface {
	// PipelineV2Enabled reports whether pipeline v2 is enabled.
	PipelineV2Enabled() bool
}

type Service struct {
	opts Options
}

var (
	_ service.Service = (*Service)(nil)
	_ Data            = (*Service)(nil)
)

// New creates a new lokipipeline service.
func New(opts Options) *Service {
	return &Service{opts: opts}
}

func (*Service) Run(ctx context.Context, _ service.Host) error {
	<-ctx.Done()
	return nil
}

func (*Service) Update(_ any) error {
	return fmt.Errorf("lokipipeline service does not support configuration")
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

func (s *Service) PipelineV2Enabled() bool {
	return s.opts.PipelineV2
}
