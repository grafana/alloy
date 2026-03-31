//go:build alloyintegrationtests

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestProcessLogFile(t *testing.T) {
	dir, err := os.Getwd()
	require.NoError(t, err)

	mountDir := filepath.Join(dir, "mount")
	t.Cleanup(func() {
		cleanupMountDir(t, mountDir)
	})

	generateFiles(t, mountDir)

	require.NoError(t, common.WaitForInitalLogs(common.SanitizeTestName(t)))

	common.AssertLogsPresent(
		t,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "cri",
				"stream": "stdout",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "cri",
				"stream": "stderr",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "docker",
				"stream": "stdout",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "json",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "logfmt",
			},
			EntryCount: 1,
		},
	)

	common.AssertLabelsNotIndexed(t, "filename")
}

func generateFiles(t *testing.T, dir string) {
	writeCRILogFile(t, dir)
	writeDockerLogFile(t, dir)
	writeJSONLogFile(t, dir)
	writeLogfmtLogFile(t, dir)
}

func writeLogFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func cleanupMountDir(t *testing.T, mountDir string) {
	t.Helper()

	entries, err := os.ReadDir(mountDir)
	require.NoError(t, err)

	for _, e := range entries {
		require.NoError(t, os.RemoveAll(filepath.Join(mountDir, e.Name())))
	}
}

func writeCRILogFile(t *testing.T, mountDir string) {
	t.Helper()

	var buf bytes.Buffer
	buf.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
	buf.WriteString(" stdout P partial stdout chunk\n")

	buf.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
	buf.WriteString(" stderr P partial stderr chunk\n")

	buf.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
	buf.WriteString(" stdout F final stdout chunk\n")

	buf.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
	buf.WriteString(" stderr P second partial stderr chunk\n")

	buf.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
	buf.WriteString(" stderr F final stderr chunk\n")

	writeLogFile(t, filepath.Join(mountDir, "cri.log"), buf.String())
}

func writeDockerLogFile(t *testing.T, mountDir string) {
	t.Helper()

	type dockerLogLine struct {
		Log    string `json:"log"`
		Stream string `json:"stream"`
		Time   string `json:"time"`
	}

	line, err := json.Marshal(dockerLogLine{
		Log:    "docker json line\n",
		Stream: "stdout",
		Time:   time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	var buf bytes.Buffer
	buf.Write(line)
	buf.WriteString("\n")
	writeLogFile(t, filepath.Join(mountDir, "docker.log"), buf.String())
}

func writeJSONLogFile(t *testing.T, mountDir string) {
	t.Helper()

	var buf bytes.Buffer
	buf.WriteString("{\"msg\":\"plain json line\"}\n")
	writeLogFile(t, filepath.Join(mountDir, "json.log"), buf.String())
}

func writeLogfmtLogFile(t *testing.T, mountDir string) {
	t.Helper()

	var buf bytes.Buffer
	buf.WriteString("msg=\"plain logfmt line\"\n")
	writeLogFile(t, filepath.Join(mountDir, "logfmt.log"), buf.String())
}
