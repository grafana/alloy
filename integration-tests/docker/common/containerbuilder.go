package common

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
)

// StartContainer starts a testcontainers container described by cfg.
// The caller is responsible for terminating the container.
func StartContainer(ctx context.Context, cfg ContainerConfig) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image: cfg.Image,
		Env:   cfg.Env,
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = cfg.Privileged
			hc.CapAdd = cfg.CapAdd
			hc.SecurityOpt = cfg.SecurityOpt
			hc.PidMode = container.PidMode(cfg.PIDMode)
		},
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}
