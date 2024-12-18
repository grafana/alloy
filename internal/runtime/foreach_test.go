package runtime_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/component-base/metrics/testutil"
)

// TODO: Test a foreach inside a foreach.
// TODO: Test foreach with clustering.
func TestForeach(t *testing.T) {
	directory := "./testdata/foreach"
	for _, file := range getTestFiles(directory, t) {
		reg := prometheus.NewRegistry()
		tc := buildTestImportFile(t, filepath.Join(directory, file.Name()))
		t.Run(tc.description, func(t *testing.T) {
			testConfigForEach(t, reg, tc.main, tc.reloadConfig, nil)
		})
	}
}

func testConfigForEach(t *testing.T, reg *prometheus.Registry, config string, reloadConfig string, update func()) {
	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, config, reg)

	err := ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ctrl.Run(ctx)
	}()

	// Check for initial condition
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		expectedMetrics := `
# HELP testcomponents_summation2 Summation of all integers received
# TYPE testcomponents_summation2 counter
testcomponents_summation2{component_id="testcomponents.summation2.final",component_path="/"} 2
`
		if err := testutil.GatherAndCompare(reg, strings.NewReader(expectedMetrics), "testcomponents_summation2_total"); err != nil {
			c.Errorf("mismatch metrics: %v", err)
		}
	}, 3*time.Second, 10*time.Millisecond)

	// if update != nil {
	// 	update()

	// 	// Export should be -10 after update
	// 	require.Eventually(t, func() bool {
	// 		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
	// 		return export.LastAdded <= -10
	// 	}, 3*time.Second, 10*time.Millisecond)
	// }

	// if reloadConfig != "" {
	// 	f, err = alloy_runtime.ParseSource(t.Name(), []byte(reloadConfig))
	// 	require.NoError(t, err)
	// 	require.NotNil(t, f)

	// 	// Reload the controller with the new config.
	// 	err = ctrl.LoadSource(f, nil)
	// 	require.NoError(t, err)

	// 	// Export should be -10 after update
	// 	require.Eventually(t, func() bool {
	// 		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
	// 		return export.LastAdded <= -10
	// 	}, 3*time.Second, 10*time.Millisecond)
	// }
}
