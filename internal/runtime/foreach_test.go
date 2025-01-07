package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

// TODO: Test a foreach inside a foreach.
// TODO: Test foreach with clustering.
func TestForeach(t *testing.T) {
	directory := "./testdata/foreach"
	for _, file := range getTestFiles(directory, t) {
		tc := buildTestForEach(t, filepath.Join(directory, file.Name()))
		t.Run(tc.description, func(t *testing.T) {
			if tc.module != "" {
				defer os.Remove("module.alloy")
				require.NoError(t, os.WriteFile("module.alloy", []byte(tc.module), 0664))
			}
			if tc.update != nil {
				testConfigForEach(t, tc.main, tc.reloadConfig, func() {
					require.NoError(t, os.WriteFile(tc.update.name, []byte(tc.update.updateConfig), 0664))
				})
			} else {
				testConfigForEach(t, tc.main, tc.reloadConfig, nil)
			}
		})
	}
}

type testForEachFile struct {
	description  string      // description at the top of the txtar file
	main         string      // root config that the controller should load
	module       string      // module imported by the root config
	reloadConfig string      // root config that the controller should apply on reload
	update       *updateFile // update can be used to update the content of a file at runtime
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
		}
	}
	return tc
}

func testConfigForEach(t *testing.T, config string, reloadConfig string, update func()) {
	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, config)

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

	require.Eventually(t, func() bool {
		sum := getDebugInfo[int](t, ctrl, "", "testcomponents.summation_receiver.sum")
		return sum >= 10
	}, 3*time.Second, 10*time.Millisecond)

	if update != nil {
		update()

		// Sum should be 30 after update
		require.Eventually(t, func() bool {
			sum := getDebugInfo[int](t, ctrl, "", "testcomponents.summation_receiver.sum")
			return sum >= 30
		}, 3*time.Second, 10*time.Millisecond)
	}

	if reloadConfig != "" {
		f, err = alloy_runtime.ParseSource(t.Name(), []byte(reloadConfig))
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
	}
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
