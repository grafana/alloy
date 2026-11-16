//go:build windows && alloyintegrationtests

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Path to the Alloy binary built by `make alloy`.
	envVarAlloyBinary = "ALLOY_BINARY"

	// Must match the label of the component in every file in configs/.
	componentLabel = "test"

	waitTimeout  = 90 * time.Second
	waitInterval = time.Second

	// A scrape of the scheduled_task collector without the default exclude reads hundreds of tasks.
	slowWaitTimeout = 4 * time.Minute

	// Task registered by the scheduled task tests. It is not below /Microsoft/.
	probeTaskName = "AlloyIntegrationTest"
)

// syncBuffer collects the process output. Alloy writes to it while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type alloyProcess struct {
	addr   string
	logs   *syncBuffer
	exited chan struct{}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().String()
}

// writeProbeFile creates a file for the file collector and returns a glob that matches it.
// The glob is escaped to be used inside an Alloy string.
func writeProbeFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "probe.txt"), []byte("alloy"), 0o600))
	return strings.ReplaceAll(filepath.Join(dir, "*.txt"), `\`, `\\`)
}

// startAlloy runs Alloy with the config in configs/<name>.alloy and stops it when the test ends.
func startAlloy(t *testing.T, name string, replacements map[string]string) *alloyProcess {
	t.Helper()

	binary := os.Getenv(envVarAlloyBinary)
	require.NotEmpty(t, binary, "%s must point to the Alloy binary", envVarAlloyBinary)
	binary, err := filepath.Abs(binary)
	require.NoError(t, err)
	require.FileExists(t, binary)

	config, err := os.ReadFile(filepath.Join("configs", name+".alloy"))
	require.NoError(t, err)
	content := string(config)
	for k, v := range replacements {
		content = strings.ReplaceAll(content, "{{"+k+"}}", v)
	}
	configPath := filepath.Join(t.TempDir(), name+".alloy")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	logs := &syncBuffer{}
	cmd := exec.CommandContext(ctx, binary, "run",
		"--server.http.listen-addr="+addr,
		"--storage.path="+t.TempDir(),
		"--disable-reporting",
		configPath,
	)
	cmd.Stdout = logs
	cmd.Stderr = logs
	// Do not wait forever for the output pipes if a child process keeps them open.
	cmd.WaitDelay = 10 * time.Second
	require.NoError(t, cmd.Start())

	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()

	t.Cleanup(func() {
		cancel()
		<-exited
		if t.Failed() {
			t.Logf("Alloy output:\n%s", logs.String())
		}
	})

	return &alloyProcess{addr: addr, logs: logs, exited: exited}
}

// metrics fetches the metrics of the exporter component.
func (p *alloyProcess) metrics() (string, error) {
	url := fmt.Sprintf("http://%s/api/v0/component/prometheus.exporter.windows.%s/metrics", p.addr, componentLabel)
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %s: %s", resp.Status, body)
	}
	return string(body), nil
}

// eventuallyMetrics scrapes until check passes or the timeout expires.
// It fails at once if Alloy exits, for example because the config is invalid.
func (p *alloyProcess) eventuallyMetrics(t *testing.T, timeout time.Duration, check func(c *assert.CollectT, metrics string)) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		select {
		case <-p.exited:
			t.Fatalf("Alloy exited early. Output:\n%s", p.logs.String())
		default:
		}
		m, err := p.metrics()
		if !assert.NoError(c, err) {
			return
		}
		check(c, m)
	}, timeout, waitInterval)
}

// registerProbeTask creates a scheduled task in the root folder and removes it when the test ends.
func registerProbeTask(t *testing.T) {
	t.Helper()
	out, err := exec.Command("schtasks", "/Create", "/TN", probeTaskName, "/SC", "ONCE", "/ST", "23:59", "/TR", "cmd.exe", "/F").CombinedOutput()
	require.NoError(t, err, "schtasks create failed: %s", out)
	t.Cleanup(func() {
		_ = exec.Command("schtasks", "/Delete", "/TN", probeTaskName, "/F").Run()
	})
}

func TestDefaultCollectors(t *testing.T) {
	p := startAlloy(t, "defaults", nil)

	p.eventuallyMetrics(t, waitTimeout, func(c *assert.CollectT, m string) {
		assert.Contains(c, m, "windows_cpu_time_seconds_total")
		assert.Contains(c, m, "windows_os_info")
		assert.Contains(c, m, `windows_exporter_collector_success{collector="cpu"} 1`)
		assert.NotContains(c, m, `windows_exporter_collector_success{collector="cpu"} 0`)
	})
}

// A config written for windows_exporter v0.31 must start, translate the renamed
// collector, and warn about the removed settings.
func TestLegacyConfigStillWorks(t *testing.T) {
	p := startAlloy(t, "legacy", map[string]string{"PROBE_GLOB": writeProbeFile(t)})

	p.eventuallyMetrics(t, waitTimeout, func(c *assert.CollectT, m string) {
		assert.Contains(c, m, "windows_cpu_time_seconds_total")
		// filetime runs the file collector.
		assert.Contains(c, m, "windows_file_size_bytes")
		assert.Contains(c, m, "probe.txt")
	})

	logs := p.logs.String()
	assert.Contains(t, logs, "the `filetime` collector and block are deprecated")
	assert.Contains(t, logs, "the `thermalzone` collector is removed")
	assert.Contains(t, logs, "the `counter_version` attribute inside the `process` block is deprecated")
}

func TestFileCollector(t *testing.T) {
	p := startAlloy(t, "file", map[string]string{"PROBE_GLOB": writeProbeFile(t)})

	p.eventuallyMetrics(t, waitTimeout, func(c *assert.CollectT, m string) {
		assert.Contains(c, m, "windows_file_mtime_timestamp_seconds")
		assert.Contains(c, m, "windows_file_size_bytes")
		assert.Contains(c, m, "probe.txt")
	})
}

func TestRegistryAndWMICollectors(t *testing.T) {
	p := startAlloy(t, "registry_wmi", nil)

	p.eventuallyMetrics(t, waitTimeout, func(c *assert.CollectT, m string) {
		assert.Contains(c, m, "windows_registry_windows_major_version")
		assert.Contains(c, m, "windows_wmi_os_processes")
		assert.Contains(c, m, `windows_exporter_collector_success{collector="registry"} 1`)
		assert.Contains(c, m, `windows_exporter_collector_success{collector="wmi"} 1`)
		// The collector success metric stays 1 when a key or query fails, so check each one.
		assert.Contains(c, m, `windows_registry_key_success{key="windows_nt"} 1`)
		assert.Contains(c, m, `windows_wmi_query_success{query="os"} 1`)
	})
}

// The default of exclude hides the tasks of Windows itself, but not other tasks.
func TestScheduledTaskDefaultExcludesMicrosoftTasks(t *testing.T) {
	registerProbeTask(t)
	p := startAlloy(t, "scheduled_task_default", nil)

	p.eventuallyMetrics(t, waitTimeout, func(c *assert.CollectT, m string) {
		assert.Contains(c, m, `windows_exporter_collector_success{collector="scheduled_task"} 1`)
		// Without this check the test passes when the collector returns no tasks at all.
		assert.Contains(c, m, `task="/`+probeTaskName+`"`)
		assert.NotContains(c, m, `task="/Microsoft/`)
	})
}

func TestScheduledTaskExcludeOverride(t *testing.T) {
	registerProbeTask(t)
	p := startAlloy(t, "scheduled_task_all", nil)

	p.eventuallyMetrics(t, slowWaitTimeout, func(c *assert.CollectT, m string) {
		assert.Contains(c, m, `windows_exporter_collector_success{collector="scheduled_task"} 1`)
		assert.Contains(c, m, `task="/`+probeTaskName+`"`)
		assert.Contains(c, m, `task="/Microsoft/`)
	})
}
