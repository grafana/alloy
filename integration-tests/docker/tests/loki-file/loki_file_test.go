//go:build alloyintegrationtests

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
	lokipipeline "github.com/grafana/alloy/integration-tests/docker/configs/loki-pipeline"
)

func TestReadLogFile(t *testing.T) {
	dir, err := os.Getwd()
	require.NoError(t, err)

	path := filepath.Join(dir, "mount", "test.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(path))
	}()

	// Write common logs to file.
	lokipipeline.GenerateLogs(t, file)
	require.NoError(t, file.Sync())
	require.NoError(t, file.Close())

	require.NoError(t, common.WaitForInitalLogs(common.SanitizeTestName(t)))

	common.AssertLogsPresent(
		t,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"stream": "stderr",
			},
			EntryCount: 1,
		},
	)
}
