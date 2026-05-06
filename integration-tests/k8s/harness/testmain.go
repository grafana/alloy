package harness

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"
)

type Options struct {
	// Name is a short identifier for the test, used in shard selection and
	// failure-diagnostics output.
	Name string
	// Dependencies is a list of dependencies to install in order. They are
	// cleaned up in reverse order.
	Dependencies []Dependency
}

// TestContext is the runtime context for a test. It holds the kubernetes
// client and registered diagnostic hooks. Namespace ownership lives in
// dependencies (e.g. deps.Namespace), not here.
type TestContext struct {
	name            string
	client          *kubernetes.Clientset
	dependencies    []Dependency
	diagnosticHooks []diagnosticHook
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

	ctx := &TestContext{
		name:   opts.Name,
		client: client,
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

func (ctx *TestContext) AddDiagnosticHook(name string, fn func(context.Context) error) {
	ctx.diagnosticHooks = append(ctx.diagnosticHooks, diagnosticHook{name: name, fn: fn})
}
