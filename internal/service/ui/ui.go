// Package ui implements the UI service.
package ui

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/web/api"
	"github.com/grafana/alloy/internal/web/ui"
)

// ServiceName defines the name used for the UI service.
const ServiceName = "ui"

// Options are used to configure the UI service. Options are constant for the
// lifetime of the UI service.
type Options struct {
	UIPrefix        string                        // Path prefix to host the UI at.
	CallbackManager livedebugging.CallbackManager // CallbackManager is used for live debugging in the UI.
}

// Service implements the UI service.
type Service struct {
	opts Options
}

// New returns a new, unstarted UI service.
func New(opts Options) *Service {
	return &Service{
		opts: opts,
	}
}

var (
	_ service.Service             = (*Service)(nil)
	_ http_service.ServiceHandler = (*Service)(nil)
)

// Definition returns the definition of the HTTP service.
func (s *Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: nil, // ui does not accept configuration
		DependsOn:  []string{http_service.ServiceName, livedebugging.ServiceName},
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

// Run starts the UI service. It will run until the provided context is
// canceled or there is a fatal error.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	<-ctx.Done()
	return nil
}

// Update implements [service.Service]. It is a no-op since the UI service
// does not support runtime configuration.
func (s *Service) Update(newConfig any) error {
	return fmt.Errorf("UI service does not support configuration")
}

// Data implements [service.Service]. It returns nil, as the UI service does
// not have any runtime data.
func (s *Service) Data() any {
	return nil
}

// ServiceHandler implements [http_service.ServiceHandler]. It returns the HTTP
// endpoints to host the UI.
func (s *Service) ServiceHandler(host service.Host) (base string, handler http.Handler) {
	r := mux.NewRouter()

	fa := api.NewAlloyAPI(host, s.opts.CallbackManager)
	fa.RegisterRoutes(path.Join(s.opts.UIPrefix, "/api/v0/web"), r)
	ui.RegisterRoutes(s.opts.UIPrefix, r)

	return s.opts.UIPrefix, r
}
