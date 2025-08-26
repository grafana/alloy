package queue

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax"
)

func TestWalCleanup(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(`
		endpoint "first" {
			url = "http://localhost:9009/api/v1/push"
		}
		endpoint "second" {
			url = "http://localhost:9009/api/v1/push"
		}
		endpoint "third" {
			url = "http://localhost:9009/api/v1/push"
		}

	`), &args)
	require.NoError(t, err)

	ctrl, err := componenttest.NewControllerFromID(logging.NewNop(), "prometheus.write.queue")
	require.NoError(t, err)

	go func() {
		require.NoError(t, ctrl.Run(componenttest.TestContext(t), args))
	}()

	ctrl.WaitRunning(5 * time.Second)

	verifyWalDirs(t, ctrl.DataPath(), []string{"first", "second", "third"})
	err = syntax.Unmarshal([]byte(`
		endpoint "first" {
			url = "http://localhost:9009/api/v1/push"
		}
		endpoint "third" {
			url = "http://localhost:9009/api/v1/push"
		}

	`), &args)
	require.NoError(t, err)

	ctrl.Update(args)

	verifyWalDirs(t, ctrl.DataPath(), []string{"first", "third"})
}

func verifyWalDirs(t *testing.T, path string, expected []string) {
	t.Helper()

	entries, err := os.ReadDir(path)
	require.NoError(t, err)

	found := map[string]bool{}
	for _, e := range entries {
		found[e.Name()] = true
	}

	require.Len(t, found, len(expected))
	for _, e := range expected {
		require.True(t, found[e])
	}
}
