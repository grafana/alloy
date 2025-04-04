package app_agent_receiver

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations/v2"
)

func init() {
	integrations.Register(&Config{}, integrations.TypeMultiplex)
}

// NewIntegration converts this config into an instance of an integration
func (c *Config) NewIntegration(l log.Logger, globals integrations.Globals) (integrations.Integration, error) {
	return nil, fmt.Errorf("app_agent_receiver integration code has been replaced by faro.receiver component")
}
