//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

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

func AlloyDarwinArm64() error {
	return buildAlloy(
		"darwin",
		"arm64",
		"netgo builtinassets",
	)
}

func AlloyDarwinAmd64() error {
	return buildAlloy(
		"darwin",
		"amd64",
		"netgo builtinassets",
	)
}

func AlloyLinuxs390x() error {
	return buildAlloy(
		"linux",
		"s390x",
		"netgo builtinassets promtail_journal_enabled",
	)
}

func AlloyLinuxPpc64le() error {
	return buildAlloy(
		"linux",
		"ppc64le",
		"netgo builtinassets promtail_journal_enabled",
	)
}

func AlloyLinuxArm64() error {
	return buildAlloy(
		"linux",
		"ppc64le",
		"netgo builtinassets promtail_journal_enabled",
	)
}

func AlloyLinuxAmd64() error {
	return buildAlloy(
		"linux",
		"amd64",
		"netgo builtinassets promtail_journal_enabled",
	)
}

func AlloyWindowsAmd64() error {
	return buildAlloy(
		"windows",
		"amd64",
		"builtinassets",
	)
}

func AlloyFreebsdAmd64() error {
	return buildAlloy(
		"freebsd",
		"amd64",
		"netgo builtinassets",
	)
}

func AlloyImage() error {
	args := map[string]string{
		"DOCKER_BUILDKIT": "1",
	}
	version, err := sh.Output("bash", "./tools/image-tag")
	if err != nil {
		return err
	}
	sh.RunV("go", "mod", "vendor")
	return sh.RunWithV(args,
		"docker",
		"build",
		"--platform", "linux/amd64",
		"--build-arg", "RELEASE_BUILD=0",
		"--build-arg", "VERSION="+version,
		"-t", "grafana/alloy:latest",
		"-f", "Dockerfile",
		".",
	)
}
