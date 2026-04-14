package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func parseOptionalDuration(s string, field string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", field, err)
	}
	return d, nil
}

// dockerHealthcheck builds a Docker API healthcheck from YAML. Test must be non-empty.
func dockerHealthcheck(h *AdditionalContainerHealthcheck) (*dockercontainer.HealthConfig, error) {
	if h == nil || len(h.Test) == 0 {
		return nil, fmt.Errorf("healthcheck.test is required")
	}
	hc := &dockercontainer.HealthConfig{Test: h.Test}

	interval, err := parseOptionalDuration(h.Interval, "healthcheck.interval")
	if err != nil {
		return nil, err
	}
	if interval > 0 {
		hc.Interval = interval
	} else {
		hc.Interval = 2 * time.Second
	}

	timeout, err := parseOptionalDuration(h.Timeout, "healthcheck.timeout")
	if err != nil {
		return nil, err
	}
	if timeout > 0 {
		hc.Timeout = timeout
	}

	startPeriod, err := parseOptionalDuration(h.StartPeriod, "healthcheck.start_period")
	if err != nil {
		return nil, err
	}
	if startPeriod > 0 {
		hc.StartPeriod = startPeriod
	}

	if h.Retries > 0 {
		hc.Retries = h.Retries
	}
	return hc, nil
}

func applyWaitOrHealthToRequest(req *testcontainers.ContainerRequest, i int, containerCfg AdditionalContainerConfig, healthStartupTimeout time.Duration) error {
	name := containerCfg.Name
	hasHealth := containerCfg.Healthcheck != nil && len(containerCfg.Healthcheck.Test) > 0

	if !hasHealth {
		return nil
	}

	hc, err := dockerHealthcheck(containerCfg.Healthcheck)
	if err != nil {
		return fmt.Errorf("additional_containers[%d] %q: %w", i, name, err)
	}
	prev := req.ConfigModifier
	req.ConfigModifier = func(c *dockercontainer.Config) {
		if prev != nil {
			prev(c)
		}
		c.Healthcheck = hc
	}
	req.WaitingFor = wait.ForHealthCheck().WithStartupTimeout(healthStartupTimeout)
	return nil
}

func startAdditionalContainers(ctx context.Context, absTestDir, networkName string, cfg TestConfig, skipImageBuild bool, testTimeout time.Duration) ([]testcontainers.Container, error) {
	healthStartupTimeout := goTestProcessTimeoutDuration(testTimeout)
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

		if err := applyWaitOrHealthToRequest(&req, i, containerCfg, healthStartupTimeout); err != nil {
			return nil, err
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

	// Start all additional containers in parallel so slow boots (e.g. Oracle) overlap
	// instead of stacking serial wall time.
	containers := make([]testcontainers.Container, len(requests))
	gctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var startErr error
	for i := range requests {
		i := i
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

	out := make([]testcontainers.Container, 0, len(containers))
	out = append(out, containers...)
	return out, nil
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
