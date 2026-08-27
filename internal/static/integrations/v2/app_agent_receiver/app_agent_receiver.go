package app_agent_receiver

import (
	"fmt"
	"log/slog"

	"github.com/grafana/alloy/internal/static/integrations/v2"
)

func init() {
	integrations.Register(&Config{}, integrations.TypeMultiplex)
}

// NewIntegration converts this config into an instance of an integration
func (c *Config) NewIntegration(_ *slog.Logger, globals integrations.Globals) (integrations.Integration, error) {
	return nil, fmt.Errorf("app_agent_receiver integration code has been replaced by faro.receiver component")
}
