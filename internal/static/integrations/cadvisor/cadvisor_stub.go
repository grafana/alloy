//go:build !linux

package cadvisor //nolint:golint

import (
	"github.com/grafana/agent/internal/static/integrations"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// NewIntegration creates a new cadvisor integration
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	level.Warn(logger).Log("msg", "the cadvisor integration only works on linux; enabling it on other platforms will do nothing")
	return &integrations.StubIntegration{}, nil
}
