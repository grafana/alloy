package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func dockerHealthcheck(h AdditionalContainerHealthcheck) (*dockercontainer.HealthConfig, error) {
	if len(h.Test) == 0 {
		return nil, fmt.Errorf("startup_healthcheck.test is required")
	}
	hc := &dockercontainer.HealthConfig{
		Test: h.Test,
	}

	if h.Interval > 0 {
		hc.Interval = h.Interval
	}

	return hc, nil
}

func startAdditionalContainers(ctx context.Context, absTestDir, networkName string, cfg TestConfig, skipImageBuild bool) ([]testcontainers.Container, error) {
	requests := make([]testcontainers.ContainerRequest, 0, len(cfg.AdditionalContainers))

	for i, containerCfg := range cfg.AdditionalContainers {
		if containerCfg.Image == "" {
			return nil, fmt.Errorf("additional_containers[%d].image must be set", i)
		}

		if containerCfg.Name == "" {
			return nil, fmt.Errorf("additional_containers[%d].name must be set", i)
		}

		req := testcontainers.ContainerRequest{
			Name:          containerCfg.Name,
			Image:         containerCfg.Image,
			ImagePlatform: integrationTestDockerPlatform,
			Env:           containerCfg.Environment,
			Cmd:           containerCfg.Command,
			Networks:      []string{networkName},
		}
		if containerCfg.StartupHealthcheck != nil {
			hc, err := dockerHealthcheck(*containerCfg.StartupHealthcheck)
			if err != nil {
				return nil, fmt.Errorf("additional_containers[%d] %q: %w", i, containerCfg.Name, err)
			}
			req.ConfigModifier = func(c *dockercontainer.Config) {
				c.Healthcheck = hc
			}
			req.WaitingFor = wait.ForHealthCheck()
		}

		if containerCfg.Build.Context != "" || containerCfg.Build.Dockerfile != "" {
			if skipImageBuild {
				fmt.Printf("skip-build: skipping additional_containers %q image build, using %s\n", containerCfg.Name, containerCfg.Image)
			} else {
				if err := buildDockerImage(absTestDir, containerCfg.Image, containerCfg.Build); err != nil {
					return nil, fmt.Errorf("failed to build additional container %q: %w", containerCfg.Name, err)
				}
			}
		}

		requests = append(requests, req)
	}

	// Start all additional containers in parallel.
	// That way containers which are slow to start won't stop others from starting up.
	containers := make([]testcontainers.Container, len(requests))
	gctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var startErr error
	for i := range requests {
		r := requests[i]
		wg.Go(func() {
			c, err := testcontainers.GenericContainer(gctx, testcontainers.GenericContainerRequest{
				ContainerRequest: r,
				Started:          true,
				Logger:           log.Default(),
			})
			if err != nil {
				mu.Lock()
				if startErr == nil {
					startErr = fmt.Errorf("failed to start additional container %q: %w", r.Name, err)
					cancel()
				}
				mu.Unlock()
				return
			}
			containers[i] = c
		})
	}
	wg.Wait()
	if startErr != nil {
		var partial []testcontainers.Container
		for _, c := range containers {
			if c != nil {
				partial = append(partial, c)
			}
		}
		_ = terminateAdditionalContainers(ctx, partial)
		return nil, startErr
	}

	return containers, nil
}

// buildDockerImage runs docker build for image using build (context and Dockerfile paths).
func buildDockerImage(absTestDir string, image string, build AdditionalContainerBuildConfig) error {
	buildContext := build.Context
	if buildContext == "" && build.Dockerfile != "" {
		buildContext = "."
	}
	if !filepath.IsAbs(buildContext) {
		buildContext = filepath.Join(absTestDir, buildContext)
	}

	args := []string{"build", "--platform", integrationTestDockerPlatform, "-t", image}
	if build.Dockerfile != "" {
		dockerfile := build.Dockerfile
		if !filepath.IsAbs(dockerfile) {
			dockerfile = filepath.Join(buildContext, dockerfile)
		}
		args = append(args, "-f", dockerfile)
	}
	args = append(args, buildContext)

	cmd := exec.Command("docker", args...)
	cmd.Dir = absTestDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building image %q: %w", image, err)
	}

	return nil
}

func terminateAdditionalContainers(ctx context.Context, containers []testcontainers.Container) error {
	var (
		wg   sync.WaitGroup
		errs = make(chan error, len(containers))
	)

	for _, c := range containers {
		wg.Go(func() {
			if err := c.Terminate(ctx); err != nil {
				errs <- err
			}
		})
	}

	wg.Wait()
	close(errs)

	var all []error
	for err := range errs {
		all = append(all, err)
	}

	return errors.Join(all...)
}
