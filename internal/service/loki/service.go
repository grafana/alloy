// Package loki implements the loki service, which exposes Loki related
// command-line options to components.
package loki

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
)

// ServiceName defines the name used for the loki service.
const ServiceName = "loki"

// Options configures the loki service.
type Options struct {
	// ConsumerPipeline enables passing logs between Loki components through
	// consumers instead of channels.
	ConsumerPipeline bool
}

// Data is the runtime data exposed by the loki service.
type Data interface {
	// ConsumerPipelineEnabled reports whether the consumer pipeline is enabled.
	ConsumerPipelineEnabled() bool
}

type Service struct {
	opts Options
}

var (
	_ service.Service = (*Service)(nil)
	_ Data            = (*Service)(nil)
)

// New creates a new loki service.
func New(opts Options) *Service {
	return &Service{opts: opts}
}

func (*Service) Run(ctx context.Context, _ service.Host) error {
	<-ctx.Done()
	return nil
}

func (*Service) Update(_ any) error {
	return fmt.Errorf("loki service does not support configuration")
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

func (s *Service) ConsumerPipelineEnabled() bool {
	return s.opts.ConsumerPipeline
}
