//go:build mage

package main

import (
	"github.com/magefile/mage/sh"
	"os"
	"strings"
)

// Test - We have to run test twice: once for all packages with -race and then once
// more without -race for packages that have known race detection issues. The
// final command runs tests for all other submodules.
func Test() error {
	namespaces, err := sh.Output("go", "list", "./...")
	if err != nil {
		return err
	}
	notIntegrationTests := make([]string, 0)
	notIntegrationTests = append(notIntegrationTests, "test")
	flags, err := buildFlags()
	if err != nil {
		return err
	}
	notIntegrationTests = append(notIntegrationTests, "-ldflags")
	notIntegrationTests = append(notIntegrationTests, strings.Join(flags, " "))
	for _, ns := range strings.Split(namespaces, "\n") {
		if strings.Contains(ns, "/integration-tests/") {
			continue
		}
		notIntegrationTests = append(notIntegrationTests, ns)
	}

	// Test race non integration tests.
	err = ExecNoEnv("non integration test", "go", notIntegrationTests...)
	if err != nil {
		return err
	}
	// Test Integrations
	err = ExecNoEnv(
		"integration test",
		"go",
		"test",
		"./internal/static/integrations/node_exporter",
		"./internal/static/logs",
		"./internal/component/otelcol/processor/tail_sampling",
		"./internal/component/loki/source/file",
		"./internal/component/loki/source/docker",
	)
	if err != nil {
		return err
	}
	// Test for race conditions in sub projects
	otherProjects, err := sh.Output("find", ".", "-name", "go.mod", "-not", "-path", "./go.mod")
	if err != nil {
		return err
	}
	wd, _ := os.Getwd()
	for _, op := range strings.Split(otherProjects, "\n") {
		os.Chdir(op)
		err = ExecNoEnv("other project test "+op, "go", "test", "-race")
		if err != nil {
			return err
		}
	}
	os.Chdir(wd)
	return nil
}

func TestPackages() error {
	err := ExecNoEnv("docker pull", "docker", "pull", buildImage)
	if err != nil {
		return err
	}
	return ExecNoEnv("test packages", "go", "test", "-tags=packaging", "./internal/tools/packaging_test")
}

func IntegrationTest() error {
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir("internal/cmd/integration-tests")
	return ExecNoEnv("integration test", "go", "run", ".")
}
