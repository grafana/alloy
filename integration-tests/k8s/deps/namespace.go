package deps

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// Namespace is a dependency that creates a Kubernetes namespace on Install
// and deletes it on Cleanup. Other dependencies that need a namespace should
// receive its name via Name().
//
// Place a Namespace first in the test's dependency list so that it is created
// before any dependencies that install resources into it. Cleanup runs in
// reverse order, so the namespace is deleted last and naturally cascades any
// stragglers.
type Namespace struct {
	opts NamespaceOptions
}

type NamespaceOptions struct {
	// Name is the name of the namespace. Required.
	Name string
	// Labels is an optional map of labels to apply to the namespace.
	Labels map[string]string
}

func NewNamespace(opts NamespaceOptions) *Namespace {
	return &Namespace{opts: opts}
}

// Name returns the namespace name. It also satisfies harness.Dependency, where
// the namespace name doubles as a clear identifier in error messages.
func (n *Namespace) Name() string {
	return n.opts.Name
}

func (n *Namespace) Install(ctx *harness.TestContext) error {
	if n.opts.Name == "" {
		return fmt.Errorf("namespace name is required")
	}
	manifest := buildNamespaceManifest(n.opts.Name, n.opts.Labels)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = commandEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apply namespace %q: %w", n.opts.Name, err)
	}
	ctx.AddDiagnosticHook("namespace "+n.opts.Name, n.diagnosticsHook())
	return nil
}

func (n *Namespace) Cleanup() {
	if n.opts.Name == "" {
		return
	}
	_ = runCommand("kubectl", "delete", "namespace", n.opts.Name,
		"--ignore-not-found=true",
		"--wait=true",
		"--timeout=10m",
	)
}

func (n *Namespace) diagnosticsHook() func(context.Context) error {
	namespace := n.opts.Name
	return func(c context.Context) error {
		return runDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", namespace, "get", "pods", "-o", "wide"},
			{"kubectl", "--namespace", namespace, "describe", "pods"},
		})
	}
}

func buildNamespaceManifest(name string, labels map[string]string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
%s`, name, indentedLabelsBlock(labels))
}

// indentedLabelsBlock renders a `labels:` block indented to live under
// `metadata:` in a Kubernetes manifest. Returns "" when there are no labels.
func indentedLabelsBlock(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	lines := []string{"  labels:"}
	for _, k := range slices.Sorted(maps.Keys(labels)) {
		lines = append(lines, fmt.Sprintf("    %s: %q", k, labels[k]))
	}
	return strings.Join(lines, "\n") + "\n"
}
