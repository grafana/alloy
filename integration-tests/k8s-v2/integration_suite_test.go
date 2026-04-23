//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/buildtag"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/planner"
)

func TestIntegrationV2(t *testing.T) {
	if activeHarness == nil {
		t.Fatal("harness not initialized")
	}
	if len(activeHarness.selectedTests) == 0 {
		t.Fatal("no selected tests")
	}
	activeHarness.runSubtests(t)
}

func (h *harness) runSubtests(t *testing.T) {
	sem := make(chan struct{}, *parallelFlag)
	for _, tc := range h.selectedTests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			sem <- struct{}{}
			defer func() { <-sem }()
			h.runSubtest(t, tc)
		})
	}
}

func (h *harness) runSubtest(t *testing.T, tc planner.TestCase) {
	testStart := time.Now()
	runtime, err := newTestRuntime(tc.Name)
	if err != nil {
		t.Fatalf("create runtime context for %s: %v", tc.Name, err)
	}
	h.log.Info("starting test", "test", tc.Name, "test_id", runtime.testID)
	workloadPath := filepath.Join(tc.Dir, "workload.yaml")
	valuesPath := filepath.Join(tc.Dir, "helm-alloy-values.yaml")
	templateVars := map[string]string{
		"TEST_ID":        runtime.testID,
		"TEST_NAMESPACE": runtime.namespace,
	}

	workloadRendered, err := renderTemplatedFile(workloadPath, templateVars)
	if err != nil {
		t.Fatalf("render workload for %s failed: %v", tc.Name, err)
	}
	defer removeTempFile(workloadRendered)

	valuesRendered, err := renderTemplatedFile(valuesPath, templateVars)
	if err != nil {
		t.Fatalf("render values for %s failed: %v", tc.Name, err)
	}
	defer removeTempFile(valuesRendered)

	// Each per-test operation (kubectl, helm) is cancelled by the test's
	// context (go test -timeout) so a hung apply/install doesn't block the
	// whole suite. Individual operations add their own tighter deadlines
	// using the readiness timeout flag.
	testCtx := t.Context()

	nsStart := time.Now()
	nsCtx, nsCancel := context.WithTimeout(testCtx, *readinessTimeout)
	err = ensureNamespace(nsCtx, h.kubeconfig, runtime.namespace)
	nsCancel()
	if err != nil {
		t.Fatalf("ensure namespace for %s failed: %v", tc.Name, err)
	}
	h.log.Info("ensure test namespace finished", "test", tc.Name, "namespace", runtime.namespace, "duration", formatStepDuration(time.Since(nsStart)))

	applyStart := time.Now()
	applyCtx, applyCancel := context.WithTimeout(testCtx, *readinessTimeout)
	err = applyWorkloadManifest(applyCtx, h.kubeconfig, workloadRendered)
	applyCancel()
	if err != nil {
		t.Fatalf("apply workload for %s failed: %v", tc.Name, err)
	}
	h.log.Info("apply workload finished", "test", tc.Name, "duration", formatStepDuration(time.Since(applyStart)))
	defer func() {
		cleanupStart := time.Now()
		// Use a detached context for cleanup: the test context may already
		// be cancelled by the time deferred cleanup runs.
		delCtx, cancel := context.WithTimeout(context.Background(), *readinessTimeout)
		defer cancel()
		if err := deleteWorkloadManifest(delCtx, h.kubeconfig, workloadRendered); err != nil {
			h.log.Warn("cleanup workload failed", "test", tc.Name, "error", err)
		}
		h.log.Info("cleanup workload finished", "test", tc.Name, "duration", formatStepDuration(time.Since(cleanupStart)))
	}()

	helmStart := time.Now()
	helmCtx, helmCancel := context.WithTimeout(testCtx, *readinessTimeout)
	err = installAlloyFromChart(helmCtx, h.kubeconfig, tc.Name, valuesRendered, runtime.release, runtime.namespace)
	helmCancel()
	if err != nil {
		t.Fatalf("install Alloy for %s failed: %v", tc.Name, err)
	}
	h.log.Info("alloy install finished", "test", tc.Name, "duration", formatStepDuration(time.Since(helmStart)))
	defer func() {
		uninstallStart := time.Now()
		uninstallCtx, cancel := context.WithTimeout(context.Background(), *readinessTimeout)
		defer cancel()
		if err := uninstallAlloyFromChart(uninstallCtx, h.kubeconfig, tc.Name, runtime.release, runtime.namespace); err != nil {
			h.log.Warn("alloy uninstall failed", "test", tc.Name, "error", err)
		}
		h.log.Info("alloy uninstall finished", "test", tc.Name, "duration", formatStepDuration(time.Since(uninstallStart)))
	}()

	reproCmd := fmt.Sprintf(
		"go test -tags %q ./integration-tests/k8s-v2 -run TestIntegrationV2/%s -args -k8s.v2.tests=%s -k8s.v2.test-id=%s",
		buildtag.Tags,
		tc.Name,
		tc.Name,
		runtime.testID,
	)
	assertStart := time.Now()
	if err := runGoTestPackage(tc.Dir, h.kubeconfig, runtime.testID); err != nil {
		t.Fatalf("test %s failed: %v\nrepro: %s", tc.Name, err, reproCmd)
	}
	h.log.Info("assertions finished", "test", tc.Name, "duration", formatStepDuration(time.Since(assertStart)))
	h.log.Info("test finished", "test", tc.Name, "duration", formatStepDuration(time.Since(testStart)))
}
