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

func runContainer(goos, goarch, tags, experiments, command string) error {
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
	image := buildImage
	if strings.Contains(experiments, "boringcrypto") {
		image = image + "-boringcrypto"
	}
	args := []string{
		"run",
		"-it",
		"--init",
		"--rm",
		"-e", "CC=viceroycc",
		"-v", wd + ":/src",
		"-w", "/src",
		// "-v", "\"alloy-build-container-gocache:/root/.cache/go-build\"",
		//"-v", "\"alloy-build-container-gomodcache:/go/pkg/mod\"",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-e", "GOOS=" + goos + "",
		"-e", "GOARCH=" + goarch + "",
		"-e", "GO_TAGS=\"" + tags + "\"",
		"-e", "GOEXPERIMENT=" + experiments,
		image,
		"./build-linux-amd64", "-v", command,
	}

	return ExecNoEnv("run container", "docker", args...)
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
	defer os.Chdir(wd)
	err := os.Chdir("./internal/web/ui")
	if err != nil {
		return err
	}
	err = sh.Run("yarn", "--network-timeout=1200000")
	if err != nil {
		return err
	}
	return sh.Run("yarn", "run", "build")

}
func buildAlloy(goos, goarch, tags, name string) error {
	return buildAlloyFull(goos, goarch, tags, name, "")
}

// buildAlloyFull builds the alloy binary with the given goos, goarch, tags and experiments, if USE_CONTAINER=1 is set
// it will run the command within a docker container.
func buildAlloyFull(goos, goarch, tags, name, experiments string) error {
	if os.Getenv("USE_CONTAINER") == "1" {
		return runContainer(goos, goarch, tags, experiments, generateMageName(goos, goarch, tags, experiments))
	}
	err := generateUI()
	if err != nil {
		return err
	}

	env := buildEnv(goos, goarch, tags, experiments)

	err = runGoMod(goos, goarch)
	if err != nil {
		return err
	}
	if name == "" {
		name = generateName(goos, goarch, tags, experiments)
	}
	// If this is a windows build then ensure we build the manifest.
	if goos == "windows" {
		winmanifestEnv := map[string]string{
			"GOOS":   "linux",
			"GOARCH": "amd64",
		}
		err = Exec("build "+name, winmanifestEnv, "go", "generate", "./internal/winmanifest")
		if err != nil {
			return err
		}
	}

	flags, err := buildFlags()
	if err != nil {
		return err
	}
	combinedFlags := strings.Join(flags, " ")
	return Exec("build "+name, env, "go", "build", "-ldflags", combinedFlags, "-tags", tags, "-o", name, ".")
}

// generateName generates the name of the binary based on the goos, goarch, tags and experiments.
func generateName(goos, goarch, tags, experiments string) string {
	name := "dist/alloy" + "-" + goos + "-" + goarch
	// If windows make it an exe file type.
	if goos == "windows" {
		name = "dist/alloy-" + goos + "-" + goarch + ".exe"
	}
	if strings.Contains(experiments, "boringcrypto") {
		name = name + "-boringcrypto"
	}
	return name
}

// generateMageName generates the name of the mage target based on the goos, experiments and goarch.
func generateMageName(goos, goarch, tags, experiments string) string {
	name := "alloy" + strcase.ToPascal(goos) + strcase.ToPascal(goarch)
	if experiments != "" {
		name = name + "BoringCrypto"
	}
	return name
}

// runGoMod runs go mod vendor in the current directory.
func runGoMod(goos, goarch string) error {
	env := buildEnv(goos, goarch, "", "")
	return Exec("update go mod", env, "go", "mod", "vendor")
}

func mapToString(env map[string]string) string {
	arr := make([]string, 0)
	for k, v := range env {
		arr = append(arr, k+"="+v)
	}
	return strings.Join(arr, " ")
}

func buildEnv(goos, goarch, tags, experiments string) map[string]string {
	env := map[string]string{
		"GOOS":        goos,
		"GOARCH":      goarch,
		"GO_TAGS":     tags,
		"CGO_ENABLED": "1",
	}
	// If not specified use the parent env.
	if env["GOOS"] == "" {
		env["GOOS"] = os.Getenv("GOOS")
	}
	if env["GOARCH"] == "" {
		env["GOARCH"] = os.Getenv("GOARCH")
	}
	if env["GO_TAGS"] == "" {
		// Use the default linux build tags.
		env["GO_TAGS"] = "netgo builtinassets promtail_journal_enabled"
	}
	if experiments != "" {
		env["GOEXPERIMENT"] = experiments
	}
	return env
}
