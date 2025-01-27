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
			testConfigForEachStringer(t, tc.main, *tc.expectedDebugInfo)
		})
	}
}

func testConfigForEachStringer(t *testing.T, config string, expectedDebugInfo string) {
	defer verifyNoGoroutineLeaks(t)
	reg := prometheus.NewRegistry()
	ctrl, f := setup(t, config, reg, featuregate.StabilityExperimental)

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

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		debugInfo := getDebugInfo[string](t, ctrl, "", "testcomponents.string_receiver.log")
		require.Equal(t, expectedDebugInfo, debugInfo)
	}, 3*time.Second, 10*time.Millisecond)
}
