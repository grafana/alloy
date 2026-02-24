// Package ui implements the UI service.
package ui

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	graphql_service "github.com/grafana/alloy/internal/service/graphql"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/livedebugging"
	remotecfg_service "github.com/grafana/alloy/internal/service/remotecfg"
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
	Logger          log.Logger
	EnableGraphQL           bool // Whether the GraphQL API is enabled.
	EnableGraphQLPlayground bool // Whether the GraphQL playground UI is enabled.
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
		DependsOn:  []string{http_service.ServiceName, livedebugging.ServiceName, remotecfg_service.ServiceName},
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
	router := mux.NewRouter()

	alloyApi := api.NewAlloyAPI(host, s.opts.CallbackManager, s.opts.Logger)
	alloyApi.RegisterRoutes(path.Join(s.opts.UIPrefix, "/api/v0/web"), router)

	if s.opts.EnableGraphQL {
		graphql_service.RegisterRoutes(s.opts.UIPrefix, router, host, s.opts.Logger, s.opts.EnableGraphQLPlayground)
	} else {
		level.Debug(s.opts.Logger).Log("msg", "GraphQL API is not enabled")
	}

	ui.RegisterRoutes(s.opts.UIPrefix, router)

	return s.opts.UIPrefix, router
}
