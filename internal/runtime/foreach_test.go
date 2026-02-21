package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime"
	_ "github.com/grafana/alloy/internal/runtime/internal/testcomponents/targets" // import targets test component
)

func TestForeach(t *testing.T) {
	directory := "./testdata/foreach"
	for _, file := range getTestFiles(directory, t) {
		tc := buildTestForEach(t, filepath.Join(directory, file.Name()))
		t.Run(file.Name(), func(t *testing.T) {
			if tc.module != "" {
				defer os.Remove("module.alloy")
				require.NoError(t, os.WriteFile("module.alloy", []byte(tc.module), 0664))
			}
			if tc.update != nil {
				testConfigForEach(t, tc.main, tc.reloadConfig, func() {
					require.NoError(t, os.WriteFile(tc.update.name, []byte(tc.update.updateConfig), 0664))
				}, nil, nil, nil)
			} else {
				testConfigForEach(t, tc.main, tc.reloadConfig, nil, nil, nil, nil)
			}
		})
	}
}

func TestForeachMetrics(t *testing.T) {
	directory := "./testdata/foreach_metrics"
	for _, file := range getTestFiles(directory, t) {
		tc := buildTestForEach(t, filepath.Join(directory, file.Name()))
		t.Run(file.Name(), func(t *testing.T) {
			if tc.module != "" {
				defer os.Remove("module.alloy")
				require.NoError(t, os.WriteFile("module.alloy", []byte(tc.module), 0664))
			}
			if tc.update != nil {
				testConfigForEach(t, tc.main, tc.reloadConfig, func() {
					require.NoError(t, os.WriteFile(tc.update.name, []byte(tc.update.updateConfig), 0664))
				}, tc.expectedMetrics, tc.expectedDurationMetrics, tc.expectedMetricsAfterReload)
			} else {
				testConfigForEach(t, tc.main, tc.reloadConfig, nil, tc.expectedMetrics, tc.expectedDurationMetrics, tc.expectedMetricsAfterReload)
			}
		})
	}
}

type testForEachFile struct {
	description                string      // description at the top of the txtar file
	main                       string      // root config that the controller should load
	module                     string      // module imported by the root config
	reloadConfig               string      // root config that the controller should apply on reload
	update                     *updateFile // update can be used to update the content of a file at runtime
	expectedMetrics            *string     // expected prometheus metrics
	expectedDurationMetrics    *int        // expected prometheus duration metrics - check those separately as they vary with each test run
	expectedDebugInfo          *string     // expected debug info after running the config
	expectedDebugInfo2         *string     // 2nd optional expected debug info after running the config
	expectedMetricsAfterReload *string     // expected prometheus metrics after reload
}

func buildTestForEach(t *testing.T, filename string) testForEachFile {
	archive, err := txtar.ParseFile(filename)
	require.NoError(t, err)
	var tc testForEachFile
	tc.description = string(archive.Comment)
	for _, alloyConfig := range archive.Files {
		switch alloyConfig.Name {
		case mainFile:
			tc.main = string(alloyConfig.Data)
		case "module.alloy":
			tc.module = string(alloyConfig.Data)
		case "update/module.alloy":
			require.Nil(t, tc.update)
			tc.update = &updateFile{
				name:         "module.alloy",
				updateConfig: string(alloyConfig.Data),
			}
		case "reload_config.alloy":
			tc.reloadConfig = string(alloyConfig.Data)
		case "expected_metrics.prom":
			expectedMetrics := string(alloyConfig.Data)
			tc.expectedMetrics = &expectedMetrics
		case "expected_metrics_after_reload.prom":
			expectedMetricsAfterReload := string(alloyConfig.Data)
			tc.expectedMetricsAfterReload = &expectedMetricsAfterReload
		case "expected_duration_metrics.prom":
			expectedDurationMetrics, err := strconv.Atoi(strings.TrimSpace(string((alloyConfig.Data))))
			require.NoError(t, err)
			tc.expectedDurationMetrics = &expectedDurationMetrics
		case "expected_debug_info.txt":
			expectedDebugInfo := string(alloyConfig.Data)
			tc.expectedDebugInfo = &expectedDebugInfo
		case "expected_debug_info2.txt":
			expectedDebugInfo2 := string(alloyConfig.Data)
			tc.expectedDebugInfo2 = &expectedDebugInfo2
		}
	}
	return tc
}

func testConfigForEach(t *testing.T, config string, reloadConfig string, update func(), expectedMetrics *string, expectedDurationMetrics *int, expectedMetricsAfterReload *string) {
	defer verifyNoGoroutineLeaks(t)
	reg := prometheus.NewRegistry()
	ctrl, f := setup(t, config, reg, featuregate.StabilityExperimental)

	err := ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Go(func() {
		ctrl.Run(ctx)
	})

	require.Eventually(t, func() bool {
		return ctrl.LoadComplete()
	}, 3*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		sum := getDebugInfo[int](t, ctrl, "", "testcomponents.summation_receiver.sum")
		return sum >= 10
	}, 3*time.Second, 10*time.Millisecond)

	if expectedDurationMetrics != nil {
		// These metrics have different values in each run.
		// Hence, we can't compare their values from run to run.
		// But we can check if the metric exists as a whole, which is good enough.
		metricsToCheck := []string{
			"alloy_component_dependencies_wait_seconds",
			"alloy_component_evaluation_seconds",
		}

		countedMetrics, err := testutil.GatherAndCount(reg, metricsToCheck...)
		require.NoError(t, err)
		require.Equal(t, *expectedDurationMetrics, countedMetrics)
	}

	if expectedMetrics != nil {
		checkMetrics(t, reg, expectedMetrics)
	}

	if update != nil {
		update()

		// Sum should be 30 after update
		require.Eventually(t, func() bool {
			sum := getDebugInfo[int](t, ctrl, "", "testcomponents.summation_receiver.sum")
			return sum >= 30
		}, 3*time.Second, 10*time.Millisecond)
	}

	if reloadConfig != "" {
		f, err = runtime.ParseSource(t.Name(), []byte(reloadConfig))
		require.NoError(t, err)
		require.NotNil(t, f)

		// Reload the controller with the new config.
		err = ctrl.LoadSource(f, nil, "")
		require.NoError(t, err)

		// Sum should be 30 after update
		require.Eventually(t, func() bool {
			sum := getDebugInfo[int](t, ctrl, "", "testcomponents.summation_receiver.sum")
			return sum >= 30
		}, 3*time.Second, 10*time.Millisecond)

		if expectedMetricsAfterReload != nil {
			checkMetrics(t, reg, expectedMetricsAfterReload)
		}
	}
}

func checkMetrics(t *testing.T, reg *prometheus.Registry, expectedMetrics *string) {
	metricsToCheck := []string{}

	// These metrics have fixed values.
	// Hence, we can compare their values from run to run.
	metrics := map[string]bool{
		"alloy_component_controller_running_components": true,
		"alloy_component_controller_evaluating":         true,
		"pulse_count":                                   true,
		// "alloy_component_evaluation_queue_size": true, // TODO - metric value is inconsistent
	}

	// Only check metrics that are present in the expected output
	for metric := range metrics {
		if strings.Contains(*expectedMetrics, metric) {
			metricsToCheck = append(metricsToCheck, metric)
		}
	}

	err := testutil.GatherAndCompare(reg, strings.NewReader(*expectedMetrics), metricsToCheck...)
	require.NoError(t, err)
}

func getDebugInfo[T any](t *testing.T, ctrl *runtime.Runtime, moduleId string, nodeId string) T {
	t.Helper()
	info, err := ctrl.GetComponent(component.ID{
		ModuleID: moduleId,
		LocalID:  nodeId,
	}, component.InfoOptions{
		GetHealth:    true,
		GetArguments: true,
		GetExports:   true,
		GetDebugInfo: true,
	})
	require.NoError(t, err)
	return info.DebugInfo.(T)
}
