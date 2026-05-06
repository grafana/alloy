package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	clusterName   = "alloy-k8s-integration"
	kubeconfigEnv = "ALLOY_TESTS_KUBECONFIG"
	managedEnv    = "ALLOY_TESTS_MANAGED_CLUSTER"
	alloyImageEnv = "ALLOY_TESTS_IMAGE"
)

type config struct {
	repoRoot      string
	kubeconfig    string
	alloyImage    string
	reuseCluster  bool
	skipAlloy     bool
	shard         string
	packageScope  string
	runRegex      string
	promOpVersion string
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := requireCommands("docker", "kind", "kubectl", "helm", "go", "make"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.Chdir(cfg.repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "change dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.kubeconfig), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create kubeconfig dir: %v\n", err)
		os.Exit(1)
	}

	cleanupNeeded := true
	defer func() {
		if !cleanupNeeded {
			return
		}
		if cfg.reuseCluster {
			logf("reuse mode enabled; keeping cluster %s", clusterName)
			return
		}
		logf("deleting kind cluster %s", clusterName)
		_ = runCommand("kind", "delete", "cluster", "--name", clusterName)
	}()

	if err := maybeBuildAlloyImage(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := ensureCluster(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := configureKubeEnv(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cleanReusedClusterNamespaces(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := loadImages(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := installPrometheusOperator(cfg.promOpVersion); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := runGoTests(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cleanupNeeded = true
}

func parseFlags() (config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return config{}, err
	}
	repoRoot := wd
	kubeconfigPath := filepath.Join(repoRoot, ".tmp", "integration-tests", "k8s", "kubeconfig")

	cfg := config{
		repoRoot:      repoRoot,
		kubeconfig:    kubeconfigPath,
		packageScope:  "./integration-tests/k8s/tests/...",
		promOpVersion: getEnv("PROM_OPERATOR_VERSION", "v0.81.0"),
	}

	// With --reuse-cluster, the runner asks the user (interactively) to confirm deletion of any
	// non-system namespaces left over from a previous run before tests start. This keeps repeated
	// local runs safe while letting you skip the kind cluster recreation cost.
	flag.BoolVar(&cfg.reuseCluster, "reuse-cluster", false, "Reuse fixed kind cluster and keep it after test run")
	flag.BoolVar(&cfg.skipAlloy, "skip-alloy-image", false, "Do not run make alloy-image (requires image to exist)")
	flag.StringVar(&cfg.shard, "shard", "", "Split test packages across shards (e.g., 0/2)")
	flag.StringVar(&cfg.packageScope, "package", cfg.packageScope, "Run one package path")
	flag.StringVar(&cfg.runRegex, "run", "", "Forward -run regex to go test")
	flag.StringVar(&cfg.alloyImage, "alloy-image", "grafana/alloy:latest", "Alloy image (repo:tag) used by tests; must exist locally or in the kind cluster")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "Usage: go run ./integration-tests/k8s/runner [flags]")
		_, _ = fmt.Fprintln(flag.CommandLine.Output())
		flag.PrintDefaults()
	}
	flag.Parse()
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

func maybeBuildAlloyImage(cfg config) error {
	if !cfg.skipAlloy {
		logf("building alloy image")
		return runCommand("make", "alloy-image")
	}
	logf("skipping alloy image build")
	return runCommandQuiet("docker", "image", "inspect", cfg.alloyImage)
}

func ensureCluster(cfg config) error {
	exists, err := clusterExists()
	if err != nil {
		return err
	}
	if exists {
		if cfg.reuseCluster {
			logf("reusing existing cluster %s", clusterName)
			return nil
		}
		logf("cluster already exists, deleting stale cluster first")
		if err := runCommand("kind", "delete", "cluster", "--name", clusterName); err != nil {
			return err
		}
		return runCommand("kind", "create", "cluster", "--name", clusterName)
	}
	logf("creating kind cluster %s", clusterName)
	return runCommand("kind", "create", "cluster", "--name", clusterName)
}

// systemNamespaces are namespaces that ship with kind/Kubernetes itself or
// that the runner installs cluster-wide (e.g. the prometheus-operator
// Deployment lives in `default`). They are never deleted by
// cleanReusedClusterNamespaces.
var systemNamespaces = map[string]struct{}{
	"default":            {},
	"kube-system":        {},
	"kube-public":        {},
	"kube-node-lease":    {},
	"local-path-storage": {},
}

// cleanReusedClusterNamespaces is a safety net for --reuse-cluster: when the
// kind cluster is being reused, any non-system namespaces from a previous
// (possibly aborted) run can produce "AlreadyExists" errors during install.
// We list them, ask the user to confirm, then delete them. No-op without
// --reuse-cluster, on a fresh cluster, or when no leftover namespaces exist.
func cleanReusedClusterNamespaces(cfg config) error {
	if !cfg.reuseCluster {
		return nil
	}
	leftovers, err := listNonSystemNamespaces(cfg.kubeconfig)
	if err != nil {
		return fmt.Errorf("list namespaces: %w", err)
	}
	if len(leftovers) == 0 {
		return nil
	}
	fmt.Println("[k8s-itest] reuse-cluster: the following non-system namespaces are leftover from a previous run and will be deleted before tests start:")
	for _, ns := range leftovers {
		fmt.Printf("  - %s\n", ns)
	}
	fmt.Print("[k8s-itest] proceed? (y/N) ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return errors.New("aborted by user; rerun without --reuse-cluster or clean the cluster manually")
	}
	for _, ns := range leftovers {
		logf("deleting namespace %s", ns)
		if err := runCommand("kubectl", "--kubeconfig", cfg.kubeconfig,
			"delete", "namespace", ns, "--wait=true", "--timeout=10m",
		); err != nil {
			return fmt.Errorf("delete namespace %q: %w", ns, err)
		}
	}
	return nil
}

func listNonSystemNamespaces(kubeconfig string) ([]string, error) {
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig,
		"get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}",
	)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("kubectl get namespaces: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var leftovers []string
	for _, ns := range strings.Fields(string(out)) {
		if _, isSystem := systemNamespaces[ns]; isSystem {
			continue
		}
		leftovers = append(leftovers, ns)
	}
	return leftovers, nil
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
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == clusterName {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func configureKubeEnv(cfg config) error {
	file, err := os.Create(cfg.kubeconfig)
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

	if err := os.Setenv(managedEnv, "1"); err != nil {
		return err
	}
	if err := os.Setenv(kubeconfigEnv, cfg.kubeconfig); err != nil {
		return err
	}
	return os.Setenv(alloyImageEnv, cfg.alloyImage)
}

func loadImages(cfg config) error {
	logf("loading required images to kind")
	if err := runCommand("kind", "load", "docker-image", cfg.alloyImage, "--name", clusterName); err != nil {
		return err
	}
	if err := runCommand("docker", "build", "-t", "prom-gen:latest", "-f", filepath.Join(cfg.repoRoot, "integration-tests/docker/configs/prom-gen/Dockerfile"), cfg.repoRoot); err != nil {
		return err
	}
	if err := runCommand("docker", "pull", "prom/blackbox-exporter:v0.25.0"); err != nil {
		return err
	}
	if err := runCommand("kind", "load", "docker-image", "prom-gen:latest", "--name", clusterName); err != nil {
		return err
	}
	return runCommand("kind", "load", "docker-image", "prom/blackbox-exporter:v0.25.0", "--name", clusterName)
}

func installPrometheusOperator(version string) error {
	logf("installing prometheus operator bundle %s", version)
	url := fmt.Sprintf("https://github.com/prometheus-operator/prometheus-operator/releases/download/%s/bundle.yaml", version)
	return runCommand("kubectl", "apply", "--kubeconfig", os.Getenv(kubeconfigEnv), "--server-side", "--validate=false", "-f", url)
}

func runGoTests(cfg config) error {
	args := []string{"test", "-v", `-tags=gore2regex`, "-timeout", "30m"}
	if cfg.runRegex != "" {
		args = append(args, "-run", cfg.runRegex)
	}
	args = append(args, cfg.packageScope)
	if cfg.shard != "" {
		logf("running shard %s", cfg.shard)
		args = append(args, "-args", "-shard="+cfg.shard)
	}
	logf("running go test %s", cfg.packageScope)
	return runCommand("go", args...)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}

func runCommandQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Env = os.Environ()
	return cmd.Run()
}

func logf(format string, args ...any) {
	fmt.Printf("[k8s-itest] "+format+"\n", args...)
}

func getEnv(name, fallback string) string {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	return v
}
