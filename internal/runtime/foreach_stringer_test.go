package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForeachStringer(t *testing.T) {
	directory := "./testdata/foreach_stringer"
	for _, file := range getTestFiles(directory, t) {
		tc := buildTestForEach(t, filepath.Join(directory, file.Name()))
		t.Run(file.Name(), func(t *testing.T) {
			if tc.module != "" {
				defer os.Remove("module.alloy")
				require.NoError(t, os.WriteFile("module.alloy", []byte(tc.module), 0664))
			}
			testConfigForEachStringer(t, tc.main, tc.expectedDebugInfo, tc.expectedDebugInfo2)
		})
	}
}

func testConfigForEachStringer(t *testing.T, config string, expectedDebugInfo *string, expectedDebugInfo2 *string) {
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

	if expectedDebugInfo != nil {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			debugInfo := getDebugInfo[string](t, ctrl, "", "testcomponents.string_receiver.log")
			assert.Equal(c, *expectedDebugInfo, debugInfo)
		}, 3*time.Second, 10*time.Millisecond)
	}

	if expectedDebugInfo2 != nil {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			debugInfo := getDebugInfo[string](t, ctrl, "", "testcomponents.string_receiver.log2")
			assert.Equal(c, *expectedDebugInfo2, debugInfo)
		}, 3*time.Second, 10*time.Millisecond)
	}
}
