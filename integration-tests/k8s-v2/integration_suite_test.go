//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegrationV2(t *testing.T) {
	if activeHarness == nil {
		t.Fatal("harness not initialized")
	}
	if len(activeHarness.selectedTests) == 0 {
		t.Fatal("no selected tests")
	}

	sem := make(chan struct{}, *parallelFlag)

	for _, tc := range activeHarness.selectedTests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			sem <- struct{}{}
			defer func() { <-sem }()

			testStart := time.Now()
			runtime, err := newTestRuntime(tc.Name)
			if err != nil {
				t.Fatalf("create runtime context for %s: %v", tc.Name, err)
			}
			activeHarness.log.Info("starting test", "test", tc.Name, "test_id", runtime.testID)
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

			nsStart := time.Now()
			if err := ensureNamespace(context.Background(), activeHarness.kubeconfig, runtime.namespace); err != nil {
				t.Fatalf("ensure namespace for %s failed: %v", tc.Name, err)
			}
			activeHarness.log.Info("ensure test namespace finished", "test", tc.Name, "namespace", runtime.namespace, "duration", formatStepDuration(time.Since(nsStart)))

			applyStart := time.Now()
			if err := applyWorkloadManifest(context.Background(), activeHarness.kubeconfig, workloadRendered); err != nil {
				t.Fatalf("apply workload for %s failed: %v", tc.Name, err)
			}
			activeHarness.log.Info("apply workload finished", "test", tc.Name, "duration", formatStepDuration(time.Since(applyStart)))
			defer func() {
				cleanupStart := time.Now()
				if err := deleteWorkloadManifest(context.Background(), activeHarness.kubeconfig, workloadRendered); err != nil {
					activeHarness.log.Warn("cleanup workload failed", "test", tc.Name, "error", err)
				}
				activeHarness.log.Info("cleanup workload finished", "test", tc.Name, "duration", formatStepDuration(time.Since(cleanupStart)))
			}()

			helmStart := time.Now()
			if err := installAlloyFromChart(context.Background(), activeHarness.kubeconfig, tc.Name, valuesRendered, runtime.release, runtime.namespace); err != nil {
				t.Fatalf("install Alloy for %s failed: %v", tc.Name, err)
			}
			activeHarness.log.Info("alloy install finished", "test", tc.Name, "duration", formatStepDuration(time.Since(helmStart)))
			defer func() {
				uninstallStart := time.Now()
				if err := uninstallAlloyFromChart(context.Background(), activeHarness.kubeconfig, tc.Name, runtime.release, runtime.namespace); err != nil {
					activeHarness.log.Warn("alloy uninstall failed", "test", tc.Name, "error", err)
				}
				activeHarness.log.Info("alloy uninstall finished", "test", tc.Name, "duration", formatStepDuration(time.Since(uninstallStart)))
			}()

			reproCmd := fmt.Sprintf(
				"go test -tags \"alloyintegrationtests k8sv2integrationtests\" ./integration-tests/k8s-v2 -run TestIntegrationV2/%s -args -k8s.v2.tests=%s -k8s.v2.test-id=%s",
				tc.Name,
				tc.Name,
				runtime.testID,
			)
			assertStart := time.Now()
			if err := runGoTestPackage(tc.Dir, activeHarness.kubeconfig, runtime.testID); err != nil {
				t.Fatalf("test %s failed: %v\nrepro: %s", tc.Name, err, reproCmd)
			}
			activeHarness.log.Info("assertions finished", "test", tc.Name, "duration", formatStepDuration(time.Since(assertStart)))
			activeHarness.log.Info("test finished", "test", tc.Name, "duration", formatStepDuration(time.Since(testStart)))
		})
	}
}
