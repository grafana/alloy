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

	"github.com/testcontainers/testcontainers-go"
)

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

	containers := make([]testcontainers.Container, 0, len(cfg.AdditionalContainers))
	for _, r := range requests {
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: r,
			Started:          true,
			Logger:           log.Default(),
		})
		if err != nil {
			_ = terminateAdditionalContainers(ctx, containers)
			return nil, fmt.Errorf("failed to start additional container %q: %w", r.Name, err)
		}

		containers = append(containers, container)
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
