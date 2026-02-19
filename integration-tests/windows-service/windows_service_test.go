//go:build windows

package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	serviceName  = "Alloy"
	registryPath = `Software\GrafanaLabs\Alloy`
	metricsURL   = "http://127.0.0.1:12345/metrics"
	waitTimeout  = 500 * time.Millisecond
	waitAttempts = 10
)

// TestWindowsService runs the Alloy Windows installer, starts the Alloy service, and uninstalls.
// Requires Administrator privileges and Windows.
// Set ALLOY_INSTALLER_PATH to the path to the built installer (e.g. dist/alloy-installer-windows-amd64.exe).
func TestWindowsService(t *testing.T) {
	installerPath := os.Getenv("ALLOY_INSTALLER_PATH")
	if installerPath == "" {
		t.Skip("ALLOY_INSTALLER_PATH not set; skipping Windows service integration test")
	}
	if _, err := os.Stat(installerPath); err != nil {
		t.Skipf("ALLOY_INSTALLER_PATH %q not found: %v", installerPath, err)
	}

	installDir := t.TempDir()
	// NSIS /D= must be last; use quoted path for spaces
	installArgs := []string{"/S", "/D=" + installDir}

	t.Logf("Running installer %s %v", installerPath, installArgs)
	cmd := exec.Command(installerPath, installArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "installer failed")

	// Installer starts the service; poll until it is running.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		ensureServiceRunning(c)
	}, waitTimeout*waitAttempts, waitTimeout)

	// Check metrics: must see alloy_component_controller_running_components in /metrics.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assertMetricsEndpoint(c)
	}, waitTimeout*waitAttempts, waitTimeout)

	// Check Windows Event Log for boringcrypto line from Alloy (logfmt: "boringcrypto enabled=false" or similar).
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assertEventLogBoringCrypto(c)
	}, waitTimeout*waitAttempts, waitTimeout)

	// Uninstall at the end.
	uninstallerPath := filepath.Join(installDir, "uninstall.exe")
	require.FileExists(t, uninstallerPath, "uninstaller should exist after install")
	t.Log("Running uninstaller /S")
	uninstall := exec.Command(uninstallerPath, "/S")
	uninstall.Stdout = os.Stdout
	uninstall.Stderr = os.Stderr
	require.NoError(t, uninstall.Run(), "uninstall failed")

	// Verify registry is clean after uninstall.
	_, err := registry.OpenKey(registry.LOCAL_MACHINE, registryPath, registry.READ)
	require.Error(t, err, "registry key %s should be deleted after uninstall", registryPath)

	// Verify install directory is deleted after uninstall.
	_, err = os.Stat(installDir)
	require.True(t, os.IsNotExist(err), "install directory %s should be deleted after uninstall", installDir)
}

// ensureServiceRunning checks that the Alloy service exists and is running.
func ensureServiceRunning(c *assert.CollectT) {
	out, err := exec.Command("sc", "query", serviceName).CombinedOutput()
	assert.NoError(c, err, "Alloy service should be running after install")
	assert.NotEmpty(c, out, "sc query returned no output")
}

// assertMetricsEndpoint fetches /metrics and requires alloy_component_controller_running_components to be present.
func assertMetricsEndpoint(c *assert.CollectT) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(metricsURL)
	assert.NoError(c, err, "metrics endpoint should be reachable")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	assert.Equal(c, http.StatusOK, resp.StatusCode, "metrics endpoint returned %s", resp.Status)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(c, err)
	assert.Contains(c, string(body), "alloy_component_controller_running_components", "metrics response missing alloy_component_controller_running_components")
}

// assertEventLogBoringCrypto checks the Windows Application event log for a recent Alloy message containing boringcrypto_enabled=false (or "boringcrypto enabled=false").
func assertEventLogBoringCrypto(c *assert.CollectT) {
	psScript := `Get-WinEvent -LogName Application -MaxEvents 500 -ErrorAction SilentlyContinue | Where-Object { $_.ProviderName -eq 'Alloy' } | ForEach-Object { $_.Message } | Out-String`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.CombinedOutput()
	assert.NoError(c, err, "Windows Event Log should contain boringcrypto_enabled=false from Alloy")
	msg := string(out)
	assert.True(c, strings.Contains(msg, "boringcrypto_enabled=false") || strings.Contains(msg, "boringcrypto enabled=false"),
		"event log did not contain boringcrypto line from Alloy; got %d bytes from Alloy events", len(msg))
}
