package harness

import (
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
	Name       string
	ConfigPath string
	Workloads  string
	Backends   []Backend
	Namespace  string
	Controller string
}

type TestContext struct {
	name                 string
	namespace            string
	mimirLocalPort       string
	alloyImageRepository string
	alloyImageTag        string
	controllerType       string
	client               *kubernetes.Clientset
	stopPortForward      func()
	diagnosticHooks      []diagnosticHook
}

func Setup(t *testing.T, opts Options) *TestContext {
	t.Helper()
	shardCheck(t, opts.Name)
	if !managedClusterEnabled() {
		t.Skip("requires managed k8s test runner; use make integration-test-k8s")
	}

	kubeconfig, err := kubeconfigFromEnv()
	if err != nil {
		t.Fatalf("%v", err)
	}
	client, err := newClient(kubeconfig)
	if err != nil {
		t.Fatalf("create kubernetes client: %v", err)
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

	ctx := &TestContext{
		name:                 opts.Name,
		namespace:            namespace,
		mimirLocalPort:       pickFreeLocalPort(),
		alloyImageRepository: imageRepo,
		alloyImageTag:        imageTag,
		controllerType:       resolveControllerType(opts.Controller),
		client:               client,
	}
	ctx.registerDiagnosticHook("namespace state", namespaceDiagnosticsHook)
	ctx.registerDiagnosticHook("alloy logs", alloyDiagnosticsHook)

	if err := ensureCleanNamespace(ctx); err != nil {
		t.Fatalf("prepare namespace: %v", err)
	}
	for _, backend := range opts.Backends {
		switch backend {
		case BackendMimir:
			if err := installMimir(ctx.namespace); err != nil {
				t.Fatalf("install mimir: %v", err)
			}
			ctx.registerDiagnosticHook("mimir logs", mimirDiagnosticsHook)
		default:
			t.Fatalf("unsupported backend %q", backend)
		}
	}

	if err := applyWorkloads(opts.Workloads); err != nil {
		t.Fatalf("apply workloads: %v", err)
	}
	if err := installAlloy(ctx, opts.ConfigPath); err != nil {
		t.Fatalf("install alloy: %v", err)
	}
	if containsBackend(opts.Backends, BackendMimir) {
		stop, err := startPortForward(ctx.namespace, ctx.mimirLocalPort)
		if err != nil {
			t.Fatalf("start mimir port-forward: %v", err)
		}
		ctx.stopPortForward = stop
	}

	return ctx
}

func (ctx *TestContext) Cleanup(t *testing.T) {
	t.Helper()

	if ctx.stopPortForward != nil {
		ctx.stopPortForward()
	}
	if t.Failed() {
		collectFailureDiagnostics(ctx)
	}
	if err := deleteNamespace(ctx.namespace); err != nil {
		t.Logf("cleanup namespace %s failed: %v", ctx.namespace, err)
	}
}

func (ctx *TestContext) Namespace() string {
	return ctx.namespace
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
		controller = "deployment"
	}
	if _, ok := valid[controller]; !ok {
		fmt.Fprintf(os.Stderr, "invalid controller type %q (expected deployment|daemonset|statefulset)\n", controller)
		os.Exit(1)
	}
	return controller
}
