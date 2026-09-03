//go:build alloyintegrationtests

package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const (
	bundleURL = "http://localhost:8089/support"
	otlpURL   = "http://localhost:4318/v1/traces"
)

// minimal OTLP/HTTP JSON trace. Pushing it makes the OTLP receiver emit its own
// internal span, which the support bundle then captures.
const otlpTrace = `{
  "resourceSpans": [{
    "scopeSpans": [{
      "spans": [{
        "traceId": "5b8efff798038103d269b633813fc60c",
        "spanId": "eee19b7ec3c1b174",
        "name": "integration-test-span",
        "kind": 1,
        "startTimeUnixNano": "1700000000000000000",
        "endTimeUnixNano": "1700000000000001000"
      }]
    }]
  }]
}`

// baseEntries are present in every bundle, regardless of the collection window.
var baseEntries = []string{
	"otelcol-support-bundle/metadata.yaml",
	"otelcol-support-bundle/pprof/heap.pprof",
	"otelcol-support-bundle/pprof/goroutine.pprof",
	"otelcol-support-bundle/traces.json",
	"otelcol-support-bundle/logs.txt",
}

// readyLog is a line the collector always writes at startup, before any bundle
// request. With a zero collection window it can only reach logs.txt through the
// prior-history ring buffer.
const readyLog = "Everything is ready. Begin running and processing data."

// TestSupportBundleOtelEngine downloads a bundle with no collection window
// (duration=0). The windowed collectors are skipped, so there is no CPU profile.
func TestSupportBundleOtelEngine(t *testing.T) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		if !assert.NoError(c, pushTrace()) {
			return
		}

		entries, err := getBundle("0")
		if !assert.NoError(c, err) {
			return
		}

		for _, e := range baseEntries {
			assert.Contains(c, entries, e)
		}
		// With no window, the CPU profile is skipped and metrics are a single scrape.
		assert.NotContains(c, entries, "otelcol-support-bundle/pprof/cpu.pprof")
		assert.Contains(c, entries, "otelcol-support-bundle/metrics.txt")
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}

// TestSupportBundleOtelEngineWithDuration downloads a bundle over a 2s window.
// The windowed collectors run, so the bundle gains a CPU profile and start/end
// metric scrapes.
func TestSupportBundleOtelEngineWithDuration(t *testing.T) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		if !assert.NoError(c, pushTrace()) {
			return
		}

		entries, err := getBundle("2")
		if !assert.NoError(c, err) {
			return
		}

		for _, e := range baseEntries {
			assert.Contains(c, entries, e)
		}
		// The window ran: a CPU profile plus paired start/end metric scrapes.
		assert.Contains(c, entries, "otelcol-support-bundle/pprof/cpu.pprof")
		assert.Contains(c, entries, "otelcol-support-bundle/metrics-start.txt")
		assert.Contains(c, entries, "otelcol-support-bundle/metrics-end.txt")
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}

// TestSupportBundleOtelEngineLogRing confirms the prior-history ring buffer
// captured logs written before the request. It uses a zero window, so the
// startup line in logs.txt can only come from the ring.
func TestSupportBundleOtelEngineLogRing(t *testing.T) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		entries, err := getBundle("0")
		if !assert.NoError(c, err) {
			return
		}
		logs, ok := entries["otelcol-support-bundle/logs.txt"]
		if !assert.True(c, ok, "logs.txt is present") {
			return
		}
		assert.Contains(c, string(logs), readyLog)
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}

// pushTrace sends one OTLP span so the collector emits internal spans.
func pushTrace() error {
	resp, err := http.Post(otlpURL, "application/json", strings.NewReader(otlpTrace))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("push trace: status %d", resp.StatusCode)
	}
	return nil
}

// getBundle downloads the bundle for the given duration query and returns its
// entries by path.
func getBundle(duration string) (map[string][]byte, error) {
	resp, err := http.Get(bundleURL + "?duration=" + duration)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bundle: status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		return nil, fmt.Errorf("bundle: content-type %q, want application/zip", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("bundle: empty body")
	}
	return unzip(body)
}

// unzip reads a zip archive and returns a map of entry path to content.
func unzip(data []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}
		out[f.Name] = content
	}
	return out, nil
}
