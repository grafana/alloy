package controller // import "go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"

import (
	"context"

	"go.opentelemetry.io/ebpf-profiler/internal/controller"
)

type Controller struct {
	*controller.Controller
}

func (c *Controller) Start(ctx context.Context) error {
	return c.Controller.Start(ctx)
}

type Config struct {
	*controller.Config
}

func (cfg *Config) Validate() error {
	return cfg.Config.Validate()
}

func New(cfg *Config) *Controller {
	return &Controller{
		controller.New(cfg.Config),
	}
}
