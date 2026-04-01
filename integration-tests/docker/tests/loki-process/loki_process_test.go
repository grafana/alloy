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
			EntryCount: 1,
			Labels: map[string]string{
				"format": "cri",
				"stream": "stdout",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/cri.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 1,
			Labels: map[string]string{
				"format": "cri",
				"stream": "stderr",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/cri.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 1,
			Labels: map[string]string{
				"format": "docker",
				"stream": "stdout",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/docker.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 2,
			Labels: map[string]string{
				"format":       "json",
				"service_name": "service-1",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/json.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 1,
			Labels: map[string]string{
				"format":       "json",
				"service_name": "service-2",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/json.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 2,
			Labels: map[string]string{
				"format":       "logfmt",
				"service_name": "service-1",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/logfmt.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 1,
			Labels: map[string]string{
				"format":       "logfmt",
				"service_name": "service-2",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/logfmt.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 1,
			Labels: map[string]string{
				"format": "nginx",
				"method": "GET",
				"status": "200",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/nginx.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 1,
			Labels: map[string]string{
				"format": "nginx",
				"method": "POST",
				"status": "201",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/nginx.log",
			},
		},
		common.ExpectedLogResult{
			EntryCount: 1,
			Labels: map[string]string{
				"format": "nginx",
				"method": "DELETE",
				"status": "404",
			},
			StructuredMetadata: map[string]string{
				"filename": "/etc/alloy/mount/nginx.log",
			},
		},
	)

	common.AssertLabelsNotIndexed(t, "filename")
}

func generateFiles(t *testing.T, dir string) {
	writeCRILogFile(t, dir)
	writeDockerLogFile(t, dir)
	writeJSONLogFile(t, dir)
	writeLogfmtLogFile(t, dir)
	writeNginxLogFile(t, dir)
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

	type jsonLogLine struct {
		Msg         string `json:"msg"`
		ServiceName string `json:"service_name"`
	}

	var buf bytes.Buffer
	writeLine := func(line jsonLogLine) {
		b, err := json.Marshal(line)
		require.NoError(t, err)
		buf.Write(b)
		buf.WriteString("\n")
	}

	writeLine(jsonLogLine{
		Msg:         "msg 1",
		ServiceName: "service-1",
	})
	writeLine(jsonLogLine{
		Msg:         "msg 2",
		ServiceName: "service-1",
	})
	writeLine(jsonLogLine{
		Msg:         "msg 3",
		ServiceName: "service-2",
	})

	writeLogFile(t, filepath.Join(mountDir, "json.log"), buf.String())
}

func writeLogfmtLogFile(t *testing.T, mountDir string) {
	t.Helper()

	var buf bytes.Buffer
	buf.WriteString("msg=\"msg 1\" service_name=\"service-1\"\n")
	buf.WriteString("msg=\"msg 2\" service_name=\"service-1\"\n")
	buf.WriteString("msg=\"msg 3\" service_name=\"service-2\"\n")
	writeLogFile(t, filepath.Join(mountDir, "logfmt.log"), buf.String())
}

func writeNginxLogFile(t *testing.T, mountDir string) {
	t.Helper()

	const dateFormat = "02/Jan/2006:15:04:05 -0700"

	var buf bytes.Buffer
	writeLine := func(remoteAddr, method, path, status, bodyBytes, userAgent string) {
		buf.WriteString(remoteAddr)
		buf.WriteString(` - - [`)
		buf.WriteString(time.Now().Format(dateFormat))
		buf.WriteString(`] "`)
		buf.WriteString(method)
		buf.WriteString(" ")
		buf.WriteString(path)
		buf.WriteString(` HTTP/1.1" `)
		buf.WriteString(status)
		buf.WriteString(" ")
		buf.WriteString(bodyBytes)
		buf.WriteString(` "-" "`)
		buf.WriteString(userAgent)
		buf.WriteString("\"\n")
	}

	writeLine("203.0.113.0", "GET", "/healthz", "200", "15", "GoogleHC/1.0")
	writeLine("203.0.113.1", "POST", "/api/v1/items", "201", "42", "curl/8.0.1")
	writeLine("203.0.113.2", "DELETE", "/api/v1/items/1", "404", "0", "curl/8.0.1")

	writeLogFile(t, filepath.Join(mountDir, "nginx.log"), buf.String())
}
