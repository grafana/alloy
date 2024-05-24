// This service provides debug stream APIs for components.
package livedebugging

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
)

// ServiceName defines the name used for the livedebugging service.
const ServiceName = "livedebugging"

type Service struct {
	liveDebugging *liveDebugging
}

var _ service.Service = (*Service)(nil)

func New() *Service {
	return &Service{
		liveDebugging: NewLiveDebugging(),
	}
}

// Data implements service.Service.
// It returns the liveDebugging for the components to stream.
func (s *Service) Data() any {
	return s.liveDebugging
}

// Definition implements service.Service.
func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: nil, // livedebugging does not accept configuration
		DependsOn:  []string{},
		Stability:  featuregate.StabilityExperimental,
	}
}

// Run implements service.Service.
func (s *Service) Run(ctx context.Context, _ service.Host) error {
	<-ctx.Done()
	return nil
}

// Update implements service.Service.
func (*Service) Update(_ any) error {
	return fmt.Errorf("livedebugging service does not support configuration")
}
