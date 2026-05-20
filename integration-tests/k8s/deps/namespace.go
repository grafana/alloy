package deps

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// Namespace creates a Kubernetes namespace on Install, deletes it on
// Cleanup. List it first so it's created before deps that target it
// and deleted last (cleanup runs in reverse).
type Namespace struct {
	opts NamespaceOptions
}

type NamespaceOptions struct {
	Name   string
	Labels map[string]string
}

func NewNamespace(opts NamespaceOptions) *Namespace {
	return &Namespace{opts: opts}
}

func (n *Namespace) Name() string {
	return n.opts.Name
}

func (n *Namespace) Install(ctx *harness.TestContext) error {
	if n.opts.Name == "" {
		return fmt.Errorf("namespace name is required")
	}
	manifest := buildNamespaceManifest(n.opts.Name, n.opts.Labels)
	if err := harness.ApplyManifest("", manifest); err != nil {
		return fmt.Errorf("apply namespace %q: %w", n.opts.Name, err)
	}
	ctx.AddDiagnosticHook("namespace "+n.opts.Name, n.diagnosticsHook())
	return nil
}

func (n *Namespace) Cleanup() {
	if n.opts.Name == "" {
		return
	}
	_ = harness.RunCommand("kubectl", "delete", "namespace", n.opts.Name,
		"--ignore-not-found=true",
		"--wait=true",
		"--timeout=10m",
	)
}

func (n *Namespace) diagnosticsHook() func(context.Context) error {
	namespace := n.opts.Name
	return func(c context.Context) error {
		return harness.RunDiagnosticCommands(c, [][]string{
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

// indentedLabelsBlock renders a `labels:` block to live under `metadata:`.
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
