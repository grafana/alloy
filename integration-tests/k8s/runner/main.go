package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
	clusterName  = "alloy-k8s-integration"
	promGenImage = "prom-gen:latest"
)

// defaultTestPackages is the fallback `go test` target when neither the
// --package flag nor the interactive picker narrows the run. It expands via
// `go list` to every package under integration-tests/k8s/tests/.
const defaultTestPackages = "./integration-tests/k8s/tests/..."

type config struct {
	repoRoot        string
	kubeconfig      string
	alloyImage      string
	reuseCluster    bool
	skipImageBuilds bool
	shard           string
	packages        []string
	interactive     bool
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := requireCommands("docker", "kind", "kubectl", "helm", "go", "make"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := os.Chdir(cfg.repoRoot); err != nil {
		fmt.Printf("change dir: %v\n", err)
		os.Exit(1)
	}
	if cfg.interactive {
		if err := configureInteractive(&cfg); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	if err := os.MkdirAll(filepath.Dir(cfg.kubeconfig), 0o700); err != nil {
		fmt.Printf("create kubeconfig dir: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if cfg.reuseCluster {
			util.Logf("reuse mode enabled; keeping cluster %s", clusterName)
			return
		}
		_ = util.Step("delete kind cluster (post-test)", func() error {
			return harness.RunCommand("kind", "delete", "cluster", "--name", clusterName)
		})
	}()

	steps := []struct {
		name string
		fn   func() error
	}{
		{"build images", func() error { return maybeBuildImages(cfg) }},
		{"ensure kind cluster", func() error { return ensureCluster(cfg) }},
		{"configure kubeconfig env", func() error { return configureEnvVariables(cfg) }},
		{"load images into kind", func() error { return loadImages(cfg) }},
		{"run go tests", func() error { return runGoTests(cfg) }},
	}
	for _, s := range steps {
		if err := util.Step(s.name, s.fn); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

func parseFlags() (config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return config{}, err
	}
	repoRoot := wd
	kubeconfigPath := filepath.Join(repoRoot, "integration-tests", "k8s", ".kube", "kubeconfig")

	cfg := config{
		repoRoot:   repoRoot,
		kubeconfig: kubeconfigPath,
	}

	fs := flag.NewFlagSet("runner", flag.ExitOnError)
	fs.SetOutput(os.Stdout)

	var pkgFlag string
	fs.BoolVar(&cfg.reuseCluster, "reuse-cluster", false, "Reuse the existing kind cluster and keep it after the run. The runner does NOT clean leftover namespaces, so a flaky previous run can fail with AlreadyExists; rerun without this flag to recreate the cluster from scratch")
	fs.BoolVar(&cfg.skipImageBuilds, "skip-image-builds", false, "Skip building local docker images (alloy, prom-gen); they must already exist in the local docker daemon. CI uses this after restoring images built in a separate job")
	fs.StringVar(&cfg.shard, "shard", "", "Split test packages across shards (e.g., 0/2)")
	fs.StringVar(&pkgFlag, "package", "", "Restrict tests to one package path or pattern (default: "+defaultTestPackages+")")
	fs.StringVar(&cfg.alloyImage, "alloy-image", "grafana/alloy:latest", "Alloy image (repo:tag) used by tests; must exist locally or in the kind cluster")
	fs.BoolVar(&cfg.interactive, "interactive", false, "Pick run options (reuse-cluster, skip-image-builds, shard/packages) via an interactive menu before running")
	fs.Usage = func() {
		fmt.Println("Usage: go run ./integration-tests/k8s/runner [flags]")
		fmt.Println()
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return config{}, err
	}
	if pkgFlag != "" {
		cfg.packages = []string{pkgFlag}
	}
	return cfg, nil
}

func requireCommands(commands ...string) error {
	for _, c := range commands {
		if _, err := exec.LookPath(c); err != nil {
			return fmt.Errorf("missing required command: %s", c)
		}
	}
	return nil
}

// maybeBuildImages builds the Alloy image and test fixture images (prom-gen).
// With --skip-image-builds it just verifies they're already in the local daemon.
func maybeBuildImages(cfg config) error {
	images := []string{cfg.alloyImage, promGenImage}
	if cfg.skipImageBuilds {
		util.Logf("--skip-image-builds set; expecting images already in local docker daemon: %v", images)
		for _, image := range images {
			if err := harness.RunCommandQuiet("docker", "image", "inspect", image); err != nil {
				return fmt.Errorf("image %q not present locally: %w", image, err)
			}
		}
		return nil
	}
	if err := util.Step("make alloy-image", func() error {
		// Pass ALLOY_IMAGE so a custom --alloy-image flag picks the right tag.
		return harness.RunCommand("make", "alloy-image", "ALLOY_IMAGE="+cfg.alloyImage)
	}); err != nil {
		return err
	}
	return util.Step("make prom-gen-image", func() error {
		return harness.RunCommand("make", "prom-gen-image")
	})
}

func ensureCluster(cfg config) error {
	exists, err := clusterExists()
	if err != nil {
		return err
	}
	if exists {
		if cfg.reuseCluster {
			util.Logf("reusing existing cluster %s", clusterName)
			return nil
		}
		util.Logf("cluster '%s' already exists, deleting stale cluster first", clusterName)
		if err := harness.RunCommand("kind", "delete", "cluster", "--name", clusterName); err != nil {
			return err
		}
	}
	return harness.RunCommand("kind", "create", "cluster", "--name", clusterName)
}

func clusterExists() (bool, error) {
	cmd := exec.Command("kind", "get", "clusters")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return false, fmt.Errorf("kind get clusters failed: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return false, err
	}
	// Match the cluster name on its own line.
	return regexp.MatchString(`(?m)^`+clusterName+`$`, string(out))
}

func configureEnvVariables(cfg config) error {
	file, err := os.OpenFile(cfg.kubeconfig, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create kubeconfig: %w", err)
	}
	defer file.Close()

	cmd := exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
	cmd.Stdout = file
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kind get kubeconfig: %w", err)
	}

	if err := os.Setenv(harness.ManagedClusterEnv, "1"); err != nil {
		return err
	}
	if err := os.Setenv(harness.KubeconfigEnv, cfg.kubeconfig); err != nil {
		return err
	}
	if err := os.Setenv(harness.AlloyImageEnv, cfg.alloyImage); err != nil {
		return err
	}
	return os.Setenv(harness.KindClusterEnv, clusterName)
}

// loadImages kind-loads the Alloy and fixture images so tests can reference
// them by name.
func loadImages(cfg config) error {
	for _, image := range []string{cfg.alloyImage, promGenImage} {
		if err := util.Step("kind load "+image, func() error {
			return harness.RunCommand("kind", "load", "docker-image", image, "--name", clusterName)
		}); err != nil {
			return err
		}
	}
	return nil
}

// runGoTests runs `go test` for the configured patterns. We intentionally
// leave package-level parallelism on (no `-p 1`): each test package owns a
// distinct namespace, so concurrent packages don't conflict on the shared
// kind cluster and we get faster wall-clock per shard. -count=1 disables
// Go's test cache so re-runs always exercise the live cluster.
func runGoTests(cfg config) error {
	patterns := cfg.packages
	if len(patterns) == 0 {
		patterns = []string{defaultTestPackages}
	}
	args := []string{"test", "-v", "-count=1", "-timeout", "30m"}
	args = append(args, patterns...)
	if cfg.shard != "" {
		args = append(args, "-args", "-shard="+cfg.shard)
	}
	return util.Step("go test "+strings.Join(patterns, " "), func() error {
		return harness.RunCommand("go", args...)
	})
}
