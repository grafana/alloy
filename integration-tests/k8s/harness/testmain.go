package harness

import (
	"context"
	"os"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
)

type Options struct {
	Name         string
	Dependencies []Dependency
	Namespace    string
}

type TestContext struct {
	name                 string
	namespace            string
	alloyImageRepository string
	alloyImageTag        string
	client               *kubernetes.Clientset
	dependencies         []Dependency
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
	if namespace == "" { // If no namespace is provided, generate a default namespace.
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
		alloyImageRepository: imageRepo,
		alloyImageTag:        imageTag,
		client:               client,
	}
	ctx.AddDiagnosticHook("namespace state", namespaceDiagnosticsHook(ctx.namespace))
	if err := ensureCleanNamespace(ctx); err != nil {
		t.Fatalf("prepare namespace: %v", err)
	}

	for _, dep := range opts.Dependencies {
		if err := dep.Install(ctx); err != nil {
			t.Fatalf("install dependency %q: %v", dep.Name(), err)
		}
		ctx.dependencies = append(ctx.dependencies, dep)
	}

	return ctx
}

func (ctx *TestContext) Cleanup(t *testing.T) {
	t.Helper()

	if t.Failed() {
		collectFailureDiagnostics(ctx)
	}
	for i := len(ctx.dependencies) - 1; i >= 0; i-- {
		ctx.dependencies[i].Cleanup()
	}
}

func (ctx *TestContext) Namespace() string {
	return ctx.namespace
}

func (ctx *TestContext) Name() string {
	return ctx.name
}

func (ctx *TestContext) AddDiagnosticHook(name string, fn func(context.Context) error) {
	ctx.diagnosticHooks = append(ctx.diagnosticHooks, diagnosticHook{name: name, fn: fn})
}

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "_", "-")
}
