//go:build mage

package main

import (
	"fmt"
	"github.com/ettle/strcase"
	"github.com/magefile/mage/sh"
	"os"
	"strings"
	"time"
)

func runContainer(goos, goarch, tags, command string) error {
	fmt.Println("running in a container")
	// Check to see if volume exists
	_, err := sh.Output("docker", "volume", "inspect", "alloy-build-container-gocache")
	goCacheExists := err == nil
	_, err = sh.Output("docker", "volume", "inspect", "alloy-build-container-gomodcache")
	goModCacheExists := err == nil
	if !goCacheExists {
		fmt.Println("building alloy-build-container-gocache")
		sh.Run("docker", "volume", "create", "alloy-build-container-gocache")
	}
	if !goModCacheExists {
		fmt.Println("building alloy-build-container-gomodcache")
		sh.Run("docker", "volume", "create", "alloy-build-container-gomodcache")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	args := make([]string, 0)
	args = append(args, "run")
	args = append(args, "-it")
	args = append(args, "--init")
	args = append(args, "--rm")
	args = append(args, "-e", "CC=viceroycc")
	args = append(args, "-v", wd+":/src")
	args = append(args, "-w", "/src")
	//args = append(args, "-v", "\"alloy-build-container-gocache:/root/.cache/go-build\"")
	//args = append(args, "-v", "\"alloy-build-container-gomodcache:/go/pkg/mod\"")
	args = append(args, "-v", "/var/run/docker.sock:/var/run/docker.sock")
	args = append(args, "-e", "GOOS="+goos+"")
	args = append(args, "-e", "GOARCH="+goarch+"")
	args = append(args, "-e", "GO_TAGS=\""+tags+"\"")
	args = append(args, "grafana/alloy-build-image:v0.1.0")
	args = append(args, "./build-linux-amd64", "-v", command)

	return sh.RunV("docker", args...)
}

func buildFlags() ([]string, error) {
	flags := make([]string, 0)
	// Need to get the version
	version, err := sh.Output("bash", "./tools/image-tag")
	if err != nil {
		return flags, err
	}
	gitRevision, err := sh.Output("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return flags, err
	}
	gitBranch, err := sh.Output("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return flags, err
	}
	user, err := sh.Output("whoami")
	if err != nil {
		return flags, err
	}
	hostname, err := sh.Output("hostname")
	if err != nil {
		return flags, err
	}
	prefix := "github.com/grafana/alloy/internal/build"
	flags = append(flags, "-X", prefix+".Branch="+gitBranch)
	flags = append(flags, "-X", prefix+".Version="+version)
	flags = append(flags, "-X", prefix+".Revision="+gitRevision)
	flags = append(flags, "-X", prefix+".BuildUser="+user+"@"+hostname)
	flags = append(flags, "-X", prefix+".BuildDate="+time.Now().UTC().Format(time.RFC3339))
	return flags, nil
}

func generateUI() error {
	fmt.Println("generating ui")
	wd, _ := os.Getwd()
	err := os.Chdir("./internal/web/ui")
	if err != nil {
		return err
	}
	err = sh.Run("yarn", "--network-timeout=1200000")
	if err != nil {
		return err
	}
	err = sh.Run("yarn", "run", "build")
	if err != nil {
		return err
	}
	return os.Chdir(wd)
}

func buildAlloy(goos, goarch, tags string) error {
	if os.Getenv("USE_CONTAINER") == "1" {
		return runContainer(goos, goarch, tags, "alloy"+strcase.ToPascal(goos)+strcase.ToPascal(goarch))
	}
	err := generateUI()
	if err != nil {
		return err
	}

	args := map[string]string{
		"GOOS":        goos,
		"GOARCH":      goarch,
		"GO_TAGS":     tags,
		"CGO_ENABLED": "1",
	}
	name := "dist/alloy" + "-" + goos + "-" + goarch
	// If windows make it an exe file type.
	if goos == "windows" {
		name = "dist/alloy-" + goos + "-" + goarch + ".exe"
		err = sh.RunV("go", "generate", "./internal/winmanifest")
		if err != nil {
			return err
		}
	}
	flags, err := buildFlags()
	if err != nil {
		return err
	}
	combinedFlags := strings.Join(flags, " ")
	return sh.RunWithV(args, "go", "build", "-v", "-ldflags", combinedFlags, "-tags", tags, "-o", name, ".")
}
