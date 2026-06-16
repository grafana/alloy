//go:build windows && alloyintegrationtests

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/windows-service/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Required environment variables.
	envVarInstallerPath = "ALLOY_INSTALLER_PATH" // path to the built installer (e.g. dist/alloy-installer-windows-amd64.exe).

	// Optional environment variables.
	envVarStateful      = "ALLOY_STATEFUL_WIN_SVC_TEST"        // if set to "true", skip cleanup (leave service installed)
	envVarCleanIfExists = "ALLOY_WIN_SVC_TEST_CLEAN_IF_EXISTS" // if set to "true", remove existing Alloy registry/service and continue

	// Constants.
	serviceName  = "Alloy"
	registryPath = `Software\GrafanaLabs\Alloy`
	metricsURL   = "http://127.0.0.1:12345/metrics"
	waitTimeout  = 500 * time.Millisecond
	waitAttempts = 10
	installDir   = `C:\Program Files\GrafanaLabs\Alloy`
	configFile   = installDir + `\config.alloy`
	dataDir      = `C:\ProgramData\GrafanaLabs\Alloy\data`
)

// TestWindowsService runs the Alloy Windows installer, starts the Alloy service, and uninstalls.
// Requires Administrator privileges and Windows.
// Set envVarInstallerPath to the path of the Alloy installer.
func TestWindowsService(t *testing.T) {
	installerPath := os.Getenv(envVarInstallerPath)
	if installerPath == "" {
		t.Fatalf("%s not set; skipping Windows service integration test", envVarInstallerPath)
	}
	if _, err := os.Stat(installerPath); err != nil {
		t.Fatalf("%s %q not found: %v", envVarInstallerPath, installerPath, err)
	}

	uninstallerPath := filepath.Join(installDir, "uninstall.exe")
	cleanup := os.Getenv(envVarStateful) != "true"
	if cleanup {
		t.Logf("Stateful mode: skipping cleanup (service will remain installed) env=%s", envVarStateful)
	} else {
		defer uninstallAlloy(t, uninstallerPath)
	}

	// Ensure no existing Alloy install; abort unless envVarCleanIfExists is set to "true".
	if isAlloyInstalled(t, installDir) {
		cleanIfExists := os.Getenv(envVarCleanIfExists) == "true"

		if !cleanIfExists {
			t.Fatalf("Alloy already present on the system. Uninstall manually or set %s=true to remove and continue", envVarCleanIfExists)
		}

		t.Logf("Alloy already present on the system. Uninstalling...")
		uninstallAlloy(t, uninstallerPath)

		if isAlloyInstalled(t, installDir) {
			t.Fatalf("Uninstall failed. Alloy is still present on the system.")
		}

		// Brief pause after cleanup before installer runs
		time.Sleep(1 * time.Second)
	}

	//TODO: Test also the "/D=" option with an obscure directory like something in TMP
	installArgs := []string{"/S", "/D=" + installDir}

	t.Logf("Running installer path=%s args=%v", installerPath, installArgs)
	cmd := exec.Command(installerPath, installArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "installer failed")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, isAlloyInstalled(t, installDir), "Alloy should be installed")
		assert.FileExists(c, uninstallerPath, "uninstaller should exist after install")
	}, waitTimeout*waitAttempts, waitTimeout)

	// TODO: Use unique component names and check for logs and metrics containing that component name.
	//       It could be a hash. This way we guarantee that we won't be using stale logs or metrics.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		util.EnsureServiceRunning(c, t, serviceName)
	}, waitTimeout*waitAttempts, waitTimeout)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		//TODO: Use a config that does something useful and check alloy_component_controller_running_components
		util.AssertMetricsEndpoint(c, metricsURL, "alloy_build_info")
	}, waitTimeout*waitAttempts, waitTimeout)

	// Check Windows Event Log for Alloy start message
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		util.AssertEventLogLine(c, "msg=\"{^_^} Alloy is running\"")
	}, waitTimeout*waitAttempts, waitTimeout)

	// Verify the installer set the expected ACLs on the install dir, config file,
	// and data dir via icacls. These are presence checks on the key ACEs rather than
	// a full golden-string match, since icacls formatting can vary across Windows builds.
	// SYSTEM must have full control; Everyone must remain read-only (and crucially must
	// NOT have full control on the secret-bearing config file).
	// TODO: If the rights tokens below don't match the runner's icacls output, adjust them
	//       against a baseline dump (see the PR verification steps), not by widening Everyone.
	// TODO: Also exercise the "/USERNAME=" install path (needs a throwaway local user via
	//       `net user`) so the $User ACL branch is covered.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Install directory (directory ACEs carry (OI)(CI) inheritance flags).
		util.AssertACLContains(c, installDir, `NT AUTHORITY\SYSTEM:(OI)(CI)(F)`, `Everyone:(OI)(CI)`)
		// Config file (a file, so no inheritance flags). Everyone must not be full.
		util.AssertACLContains(c, configFile, `NT AUTHORITY\SYSTEM:(F)`, `Everyone:`)
		acl, err := util.GetACL(configFile)
		assert.NoError(c, err)
		assert.NotContains(c, acl, `Everyone:(F)`, "Everyone must not have full control of the config file")
		// Data directory.
		util.AssertACLContains(c, dataDir, `NT AUTHORITY\SYSTEM:(OI)(CI)(F)`, `Everyone:(OI)(CI)`)
	}, waitTimeout*waitAttempts, waitTimeout)
}

func isAlloyInstalled(t *testing.T, installDir string) bool {
	_, err := os.Stat(installDir)
	filesExist := !os.IsNotExist(err)

	registryKeyExists := util.RegistryKeyExists(registryPath)
	serviceExists := util.ServiceExists(serviceName)

	t.Logf("Checking if Alloy is installed: files=%v registry=%v service=%v", filesExist, registryKeyExists, serviceExists)
	return filesExist && registryKeyExists && serviceExists
}

func uninstallAlloy(t *testing.T, uninstallerPath string) {
	t.Logf("Running uninstaller")
	uninstall := exec.Command(uninstallerPath, "/S")
	uninstall.Stdout = os.Stdout
	uninstall.Stderr = os.Stderr
	require.NoError(t, uninstall.Run(), "uninstall failed")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, isAlloyInstalled(t, installDir), "Alloy should be uninstalled")
	}, waitTimeout*waitAttempts, waitTimeout)
}
