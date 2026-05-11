// Package harness wires Go tests to the runner-managed kind cluster. Tests
// call Setup with a list of Dependencies; Setup installs them in order and
// registers a t.Cleanup tearing them down in reverse (parallel-subtest safe).
package harness

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/util"
)

// Dependency is implemented by every test fixture. Install must block
// until the dep is usable so "Install returned" means "ready". Cleanup
// is best-effort.
type Dependency interface {
	Name() string
	Install(*TestContext) error
	Cleanup()
}

type Options struct {
	// Dependencies are installed in order, cleaned up in reverse.
	Dependencies []Dependency
}

// TestContext tracks installed deps and diagnostic hooks for a single test.
type TestContext struct {
	name            string
	pkgPath         string
	dependencies    []Dependency
	diagnosticHooks []diagnosticHook
}

func Setup(t *testing.T, opts Options) *TestContext {
	// runtime.Caller(1) must run first so the frame still points at the
	// test file; any helper above this line would shift the frame.
	_, callerFile, _, _ := runtime.Caller(1)

	t.Helper()

	// Shard by package — see harness/shard.go.
	pkgPath := derivePkgPath(callerFile)
	shardCheck(t, pkgPath)

	if !managedClusterEnabled() {
		t.Skip("requires managed k8s test runner; use make integration-test-k8s")
	}

	if _, err := kubeconfigFromEnv(); err != nil {
		t.Fatalf("%v", err)
	}

	ctx := &TestContext{
		name:    t.Name(),
		pkgPath: pkgPath,
	}

	// Register cleanup before installing so partial-install failures still
	// tear down. t.Cleanup (not defer) is what makes parallel subtests safe.
	t.Cleanup(func() { ctx.cleanup(t) })

	for _, dep := range opts.Dependencies {
		err := util.Step("install dep "+dep.Name(), func() error { return dep.Install(ctx) })
		if err != nil {
			t.Fatalf("install dependency %q: %v", dep.Name(), err)
		}
		ctx.dependencies = append(ctx.dependencies, dep)
	}

	return ctx
}

// derivePkgPath returns the repo-rooted package path (e.g.
// "integration-tests/k8s/tests/foo") for the failure-diagnostics repro hint.
func derivePkgPath(callerFile string) string {
	if callerFile == "" {
		return ""
	}
	const marker = "integration-tests/"
	if idx := strings.Index(callerFile, marker); idx >= 0 {
		return filepath.Dir(callerFile[idx:])
	}
	return filepath.Dir(callerFile)
}

func (ctx *TestContext) cleanup(t *testing.T) {
	t.Helper()

	if t.Failed() {
		collectFailureDiagnostics(ctx)
	}
	for i := len(ctx.dependencies) - 1; i >= 0; i-- {
		dep := ctx.dependencies[i]
		_ = util.Step("cleanup dep "+dep.Name(), func() error {
			dep.Cleanup()
			return nil
		})
	}
}

func (ctx *TestContext) AddDiagnosticHook(name string, fn func(context.Context) error) {
	ctx.diagnosticHooks = append(ctx.diagnosticHooks, diagnosticHook{name: name, fn: fn})
}
