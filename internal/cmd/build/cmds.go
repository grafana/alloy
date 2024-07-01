//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// AlloyBuildAll builds the all the dist targets.
func AlloyBuildAll() {
	mg.SerialDeps(
		AlloyDarwinArm64,
		AlloyDarwinAmd64,
		AlloyLinuxArm64,
		AlloyLinuxAmd64,
		AlloyLinuxs390x,
		AlloyLinuxPpc64le,
		AlloyWindowsAmd64,
		AlloyFreebsdAmd64,
	)
}

// Alloy builds the current OS and ARCH system to `build/alloy`.
func Alloy() error {
	return buildAlloy("", "", "", "build/alloy")
}

// AlloyGoMod runs `go mod vendor` in the current directory.
func AlloyGoMod() error {
	return runGoMod("", "")
}

// AlloyImage builds the linux amd64 docker image for alloy.
func AlloyImage() error {
	args := map[string]string{
		"DOCKER_BUILDKIT": "1",
	}
	version, err := sh.Output("bash", "./tools/image-tag")
	if err != nil {
		return err
	}
	err = ExecNoEnv("build image", "go", "mod", "vendor")
	if err != nil {
		return err
	}
	return Exec("build image", args,
		"docker",
		"build",
		"--platform", "linux/amd64",
		"--build-arg", "RELEASE_BUILD=0",
		"--build-arg", "VERSION="+version,
		"--build-arg", "BUILDPLATFORM=linux/amd64",
		"--build-arg", "TARGETOS=linux",
		"--build-arg", "TARGETARCH=amd64",
		"-t", "grafana/alloy:latest",
		"-f", "Dockerfile",
		".",
	)
}

// AlloyImageBoringCrypto builds the linux amd64 docker image for alloy with boring crypto.
func AlloyImageBoringCrypto() error {
	args := map[string]string{
		"DOCKER_BUILDKIT": "1",
	}
	version, err := sh.Output("bash", "./tools/image-tag")
	if err != nil {
		return err
	}
	err = ExecNoEnv("build image", "go", "mod", "vendor")
	if err != nil {
		return err
	}
	return Exec("build image", args,
		"docker",
		"build",
		"--platform", "linux/amd64",
		"--build-arg", "RELEASE_BUILD=0",
		"--build-arg", "VERSION="+version,
		"--build-arg", "BUILDPLATFORM=linux/amd64",
		"--build-arg", "TARGETOS=linux",
		"--build-arg", "TARGETARCH=amd64",
		"--build-arg", "GOEXPERIMENT=boringcrypto",
		"-t", "grafana/alloy:boringcrypto",
		"-f", "Dockerfile",
		".",
	)
}

// AlloyImageWindows builds the windows amd64 docker image for alloy.
func AlloyImageWindows() error {
	args := map[string]string{
		"DOCKER_BUILDKIT": "1",
	}
	version, err := sh.Output("bash", "./tools/image-tag")
	if err != nil {
		return err
	}
	err = ExecNoEnv("build image", "go", "mod", "vendor")
	if err != nil {
		return err
	}
	return Exec("build image", args,
		"docker",
		"build",
		"--platform", "windows/amd64",
		"--build-arg", "RELEASE_BUILD=0",
		"--build-arg", "VERSION="+version,
		"--build-arg", "BUILDPLATFORM=windows/amd64",
		"--build-arg", "TARGETOS=windows",
		"--build-arg", "TARGETARCH=amd64",
		"-t", "grafana/alloy:nanoserver-1809",
		"-f", "Dockerfile.windows",
		".",
	)
}
