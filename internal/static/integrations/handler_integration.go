package integrations

import (
	"context"
	"net/http"

	"github.com/grafana/agent/internal/static/integrations/config"
)

// NewHandlerIntegration creates a new named integration that will call handler
// when metrics are needed.
func NewHandlerIntegration(name string, handler http.Handler) Integration {
	return &handlerIntegration{name: name, handler: handler}
}

type handlerIntegration struct {
	name    string
	handler http.Handler
}

func (hi *handlerIntegration) MetricsHandler() (http.Handler, error) {
	return hi.handler, nil
}

func (hi *handlerIntegration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     hi.name,
		MetricsPath: "/metrics",
	}}
}

func (hi *handlerIntegration) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
