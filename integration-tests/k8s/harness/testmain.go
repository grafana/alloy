package harness

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
)

type Backend string

const (
	BackendMimir Backend = "mimir"
)

type Options struct {
	Name         string
	ConfigPath   string
	Workloads    string
	Backends     []Backend
	PodWaits     []string
	Namespace    string
	AlloyRelease string
	Controller   string
}

type TestContext struct {
	Name                 string
	Namespace            string
	TestID               string
	MimirLocalPort       string
	Shard                shardConfig
	AlloyImageRepository string
	AlloyImageTag        string
	ControllerType       string
	client               *kubernetes.Clientset
}

var current *TestContext

func Current(t *testing.T) *TestContext {
	t.Helper()
	if current == nil {
		t.Fatalf("harness is not initialized, use harness.RunTestMain in TestMain")
	}
	return current
}

func RunTestMain(m *testing.M, opts Options) {
	flag.Parse()

	shard, err := parseShard()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid shard flag: %v\n", err)
		os.Exit(1)
	}
	if !shard.shouldRun(opts.Name) {
		fmt.Printf("[k8s-itest] skipping package %s for shard %s\n", opts.Name, *shardFlag)
		os.Exit(0)
	}

	kubeconfig, err := kubeconfigFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client, err := newClient(kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create kubernetes client: %v\n", err)
		os.Exit(1)
	}

	namespace := opts.Namespace
	if namespace == "" {
		namespace = "test-" + sanitizeName(opts.Name)
	}

	image := os.Getenv("ALLOY_IMAGE")
	imageRepo := "grafana/alloy"
	imageTag := "latest"
	if image != "" {
		if idx := strings.LastIndex(image, ":"); idx > 0 {
			imageRepo = image[:idx]
			imageTag = image[idx+1:]
		}
	}

	current = &TestContext{
		Name:                 opts.Name,
		Namespace:            namespace,
		TestID:               opts.Name,
		MimirLocalPort:       pickFreeLocalPort(),
		Shard:                shard,
		AlloyImageRepository: imageRepo,
		AlloyImageTag:        imageTag,
		ControllerType:       resolveControllerType(opts.Controller),
		client:               client,
	}

	if err := ensureCleanNamespace(current); err != nil {
		fmt.Fprintf(os.Stderr, "prepare namespace: %v\n", err)
		os.Exit(1)
	}
	for _, backend := range opts.Backends {
		switch backend {
		case BackendMimir:
			if err := installMimir(current.Namespace); err != nil {
				fmt.Fprintf(os.Stderr, "install mimir: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "unsupported backend %q\n", backend)
			os.Exit(1)
		}
	}

	if err := applyWorkloads(opts.Workloads, current.Namespace); err != nil {
		fmt.Fprintf(os.Stderr, "apply workloads: %v\n", err)
		os.Exit(1)
	}
	if err := installAlloy(current, opts.ConfigPath); err != nil {
		fmt.Fprintf(os.Stderr, "install alloy: %v\n", err)
		os.Exit(1)
	}
	var stopPortForward func()
	if containsBackend(opts.Backends, BackendMimir) {
		stop, err := startPortForward(current.Namespace, current.MimirLocalPort)
		if err != nil {
			fmt.Fprintf(os.Stderr, "start mimir port-forward: %v\n", err)
			os.Exit(1)
		}
		stopPortForward = stop
	}

	exitCode := m.Run()
	if stopPortForward != nil {
		stopPortForward()
	}
	if exitCode != 0 {
		collectFailureDiagnostics(current)
	}
	os.Exit(exitCode)
}

func containsBackend(backends []Backend, backend Backend) bool {
	for _, b := range backends {
		if b == backend {
			return true
		}
	}
	return false
}

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "_", "-")
}

func pickFreeLocalPort() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "22021"
	}
	defer l.Close()
	parts := strings.Split(l.Addr().String(), ":")
	return parts[len(parts)-1]
}

func repoRootFromCwd() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", fmt.Errorf("unable to find repo root from %s", dir)
		}
		dir = next
	}
}

func resolveControllerType(optionValue string) string {
	valid := map[string]struct{}{
		"deployment":  {},
		"daemonset":   {},
		"statefulset": {},
	}

	controller := optionValue
	if controller == "" {
		controller = os.Getenv("ALLOY_K8S_CONTROLLER_TYPE")
	}
	if controller == "" {
		controller = "deployment"
	}
	if _, ok := valid[controller]; !ok {
		fmt.Fprintf(os.Stderr, "invalid controller type %q (expected deployment|daemonset|statefulset)\n", controller)
		os.Exit(1)
	}
	return controller
}
