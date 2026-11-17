//go:build !nonetwork && !nodocker && packaging

package packaging_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
)

// TestAlloyLinuxPackages runs the entire test suite for the Linux packages.
func TestAlloyLinuxPackages(t *testing.T) {
	packageName := "alloy"

	fmt.Println("Building packages (this may take a while...)")
	buildAlloyPackages(t)

	dockerPool, err := dockertest.NewPool("")
	require.NoError(t, err)

	tt := []struct {
		name string
		f    func(*AlloyEnvironment, *testing.T)
	}{
		{"install package", (*AlloyEnvironment).TestInstall},
		{"ensure existing config doesn't get overridden", (*AlloyEnvironment).TestConfigPersistence},
		{"test data folder permissions", (*AlloyEnvironment).TestDataFolderPermissions},
		{"test engine toggle", (*AlloyEnvironment).TestEngineToggle},
		{"test systemd service", (*AlloyEnvironment).TestSystemdService},
	}

	for _, tc := range tt {
		t.Run(tc.name+"/rpm", func(t *testing.T) {
			env := &AlloyEnvironment{RPMEnvironment(t, packageName, dockerPool)}
			tc.f(env, t)
		})
		t.Run(tc.name+"/deb", func(t *testing.T) {
			env := &AlloyEnvironment{DEBEnvironment(t, packageName, dockerPool)}
			tc.f(env, t)
		})
	}
}

func buildAlloyPackages(t *testing.T) {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	root, err := filepath.Abs(filepath.Join(wd, "../../.."))
	require.NoError(t, err)

	cmd := exec.Command("make", fmt.Sprintf("dist-alloy-packages-%s", runtime.GOARCH))
	cmd.Env = append(
		os.Environ(),
		"VERSION=v0.0.0",
		"DOCKER_OPTS=",
	)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
}

type AlloyEnvironment struct{ Environment }

func (env *AlloyEnvironment) TestInstall(t *testing.T) {
	res := env.Install()
	require.Equal(t, 0, res.ExitCode, "installing failed")

	res = env.ExecScript(`[ -f /usr/bin/alloy ]`)
	require.Equal(t, 0, res.ExitCode, "expected Alloy to be installed")
	res = env.ExecScript(`[ -f /etc/alloy/config.alloy ]`)
	require.Equal(t, 0, res.ExitCode, "expected Alloy configuration file to exist")

	res = env.ExecScript(`stat -c '%a:%U:%G' /etc/alloy`)
	require.Equal(t, "770:root:alloy\n", res.Stdout, "wrong permissions for config folder")
	require.Equal(t, 0, res.ExitCode, "stat'ing config folder failed")

	res = env.Uninstall()
	require.Equal(t, 0, res.ExitCode, "uninstalling failed")

	res = env.ExecScript(`[ -f /usr/bin/alloy ]`)
	require.Equal(t, 1, res.ExitCode, "expected Alloy to be uninstalled")
	// NOTE(rfratto): we don't check for what happens to the config file here,
	// since the behavior is inconsistent: rpm uninstalls it, but deb doesn't.
}

func (env *AlloyEnvironment) TestConfigPersistence(t *testing.T) {
	res := env.ExecScript(`mkdir -p /etc/alloy`)
	require.Equal(t, 0, res.ExitCode, "failed to create config directory")

	res = env.ExecScript(`echo -n "keepalive" > /etc/alloy/config.alloy`)
	require.Equal(t, 0, res.ExitCode, "failed to write config file")

	res = env.ExecScript(`echo -n "keepalive-otel" > /etc/alloy/config.yaml`)
	require.Equal(t, 0, res.ExitCode, "failed to write otel config file")

	res = env.Install()
	require.Equal(t, 0, res.ExitCode, "installation failed")

	res = env.ExecScript(`cat /etc/alloy/config.alloy`)
	require.Equal(t, "keepalive", res.Stdout, "Expected existing file to not be overridden")

	res = env.ExecScript(`cat /etc/alloy/config.yaml`)
	require.Equal(t, "keepalive-otel", res.Stdout, "Expected existing OTel config to not be overridden")
}

func (env *AlloyEnvironment) TestEngineToggle(t *testing.T) {
	res := env.Install()
	require.Equal(t, 0, res.ExitCode, "installation failed")

	res = env.ExecScript(`grep -q 'ExecStart=/usr/lib/alloy/alloy-wrapper' /usr/lib/systemd/system/alloy.service`)
	require.Equal(t, 0, res.ExitCode, "expected the unit file to exec the wrapper script")

	res = env.ExecScript(`[ -x /usr/lib/alloy/alloy-wrapper ]`)
	require.Equal(t, 0, res.ExitCode, "expected the wrapper script to be installed and executable")

	res = env.ExecScript(`[ -f /etc/alloy/config.yaml ]`)
	require.Equal(t, 0, res.ExitCode, "expected the default OTel engine config to be installed")

	res = env.ExecScript(`f=/etc/default/alloy; [ -f "$f" ] || f=/etc/sysconfig/alloy; grep -qF 'ALLOY_OTEL_MODE=""' "$f"`)
	require.Equal(t, 0, res.ExitCode, "expected the installed environment file to declare ALLOY_OTEL_MODE as empty by default")

	res = env.ExecScript(`f=/etc/default/alloy; [ -f "$f" ] || f=/etc/sysconfig/alloy; grep -qF 'OTEL_CONFIG_FILE="/etc/alloy/config.yaml"' "$f"`)
	require.Equal(t, 0, res.ExitCode, "expected the installed environment file to declare OTEL_CONFIG_FILE with its default value")

	// Install a simple util that can help us determine the bounds of argv parameters passed to the alloy binary
	// It's important that we test this as we're relying on the alloy-wrapper to split argv's for us
	res = env.ExecScript(`cat > /tmp/argv <<'SHIM'
#!/bin/sh
for a in "$@"; do printf '[%s]' "$a"; done
printf '\n'
SHIM
chmod +x /tmp/argv`)
	require.Equal(t, 0, res.ExitCode, "failed to install the argv shim")

	tt := []struct {
		name     string
		env      string
		expected string
	}{
		{
			name:     "default engine, unset toggle",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy`,
			expected: "[run][--storage.path=/var/lib/alloy/data][/etc/alloy/config.alloy]\n",
		},
		{
			name:     "default engine, custom CONFIG_FILE",
			env:      `CONFIG_FILE=/custom/config.alloy`,
			expected: "[run][--storage.path=/var/lib/alloy/data][/custom/config.alloy]\n",
		},
		{
			name:     "otel engine, default config",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy ALLOY_OTEL_MODE=1`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine, ALLOY_OTEL_MODE=true",
			env:      `ALLOY_OTEL_MODE=true`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine, ALLOY_OTEL_MODE=yes",
			env:      `ALLOY_OTEL_MODE=yes`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine, ALLOY_OTEL_MODE=on",
			env:      `ALLOY_OTEL_MODE=on`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine, ALLOY_OTEL_MODE matching is case-insensitive (TRUE)",
			env:      `ALLOY_OTEL_MODE=TRUE`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine, ALLOY_OTEL_MODE matching is case-insensitive (Yes)",
			env:      `ALLOY_OTEL_MODE=Yes`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine, ALLOY_OTEL_MODE matching is case-insensitive (ON)",
			env:      `ALLOY_OTEL_MODE=ON`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine ignores CONFIG_FILE, the default engine's config path",
			env:      `CONFIG_FILE=/custom/config.alloy ALLOY_OTEL_MODE=1`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "otel engine, custom OTEL_CONFIG_FILE",
			env:      `ALLOY_OTEL_MODE=1 OTEL_CONFIG_FILE=/custom/config.yaml`,
			expected: "[otel][--config=/custom/config.yaml]\n",
		},
		{
			name:     "default engine ignores OTEL_CONFIG_FILE, the OTel engine's config path",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy OTEL_CONFIG_FILE=/custom/config.yaml`,
			expected: "[run][--storage.path=/var/lib/alloy/data][/etc/alloy/config.alloy]\n",
		},
		{
			name:     "otel engine, OTEL_CUSTOM_ARGS applies",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy ALLOY_OTEL_MODE=1 OTEL_CUSTOM_ARGS="--feature-gates=otelcol.printInitialConfig"`,
			expected: "[otel][--config=/etc/alloy/config.yaml][--feature-gates=otelcol.printInitialConfig]\n",
		},
		{
			name:     "otel engine ignores CUSTOM_ARGS, the default engine's flags",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy ALLOY_OTEL_MODE=1 CUSTOM_ARGS="--storage.path=/should/not/appear"`,
			expected: "[otel][--config=/etc/alloy/config.yaml]\n",
		},
		{
			name:     "default engine ignores OTEL_CUSTOM_ARGS, the OTel engine's flags",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy OTEL_CUSTOM_ARGS="--feature-gates=otelcol.printInitialConfig"`,
			expected: "[run][--storage.path=/var/lib/alloy/data][/etc/alloy/config.alloy]\n",
		},
		{
			name:     "default engine, CUSTOM_ARGS with multiple flags and a glob character is split and passed through literally",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy CUSTOM_ARGS="--set=/etc/alloy/* --disable-reporting"`,
			expected: "[run][--set=/etc/alloy/*][--disable-reporting][--storage.path=/var/lib/alloy/data][/etc/alloy/config.alloy]\n",
		},
		{
			name:     "otel engine, OTEL_CUSTOM_ARGS with multiple flags and a glob character is split and passed through literally",
			env:      `CONFIG_FILE=/etc/alloy/config.alloy ALLOY_OTEL_MODE=1 OTEL_CUSTOM_ARGS="--set=/etc/alloy/* --feature-gates=otelcol.printInitialConfig"`,
			expected: "[otel][--config=/etc/alloy/config.yaml][--set=/etc/alloy/*][--feature-gates=otelcol.printInitialConfig]\n",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res := env.ExecScript(fmt.Sprintf(`ALLOY_BIN=/tmp/argv %s /usr/lib/alloy/alloy-wrapper`, tc.env))
			require.Equal(t, 0, res.ExitCode, "wrapper script exited non-zero")
			require.Equal(t, tc.expected, res.Stdout)
		})
	}
}

// TestSystemdService verifies that systemd starts Alloy from the installed unit
// file, and that toggling ALLOY_OTEL_MODE in the environment file changes which
// engine systemd launches.
func (env *AlloyEnvironment) TestSystemdService(t *testing.T) {
	env.logSystemdDiagnosticsOnFailure(t)

	res := env.Install()
	require.Equalf(t, 0, res.ExitCode, "installation failed:\n%s%s", res.Stdout, res.Stderr)

	res = env.ExecScript(`systemctl daemon-reload && systemctl enable --now alloy`)
	require.Equal(t, 0, res.ExitCode, "failed to enable and start the alloy service")

	// An unset ALLOY_OTEL_MODE keeps the default engine, so a fresh install must
	// behave the same as it did before the toggle existed.
	env.requireServiceActive(t)
	env.requireAlloyArgs(t, "run --storage.path=/var/lib/alloy/data /etc/alloy/config.alloy")

	res = env.ExecScript(`f=/etc/default/alloy; [ -f "$f" ] || f=/etc/sysconfig/alloy; sed -i 's/^ALLOY_OTEL_MODE=.*/ALLOY_OTEL_MODE="1"/' "$f"`)
	require.Equal(t, 0, res.ExitCode, "failed to enable ALLOY_OTEL_MODE in the environment file")

	res = env.ExecScript(`systemctl restart alloy`)
	require.Equal(t, 0, res.ExitCode, "failed to restart the alloy service")

	env.requireServiceActive(t)
	env.requireAlloyArgs(t, "otel --config=/etc/alloy/config.yaml")
}

// requireServiceActive waits for the alloy service to become active, then checks
// that it stayed up
func (env *AlloyEnvironment) requireServiceActive(t *testing.T) {
	t.Helper()

	var state string
	deadline := time.Now().Add(serviceStartTimeout)
	for time.Now().Before(deadline) {
		state = strings.TrimSpace(env.ExecScript(`systemctl is-active alloy`).Stdout)
		if state == "active" {
			break
		}
		time.Sleep(pollInterval)
	}
	require.Equal(t, "active", state, "alloy service never became active")

	time.Sleep(serviceSettleTime)

	state = strings.TrimSpace(env.ExecScript(`systemctl is-active alloy`).Stdout)
	require.Equal(t, "active", state, "alloy service did not stay active")

	res := env.ExecScript(`systemctl show -p NRestarts --value alloy`)
	require.Equal(t, "0", strings.TrimSpace(res.Stdout), "alloy service failed to boot successfully at least once")
}

// requireAlloyArgs asserts which command systemd actually launched.
func (env *AlloyEnvironment) requireAlloyArgs(t *testing.T, expectedArgs string) {
	t.Helper()

	res := env.ExecScript(`ps -o args= -C alloy | head -n 1`)
	require.Equal(t, "/usr/bin/alloy "+expectedArgs, strings.TrimSpace(res.Stdout),
		"systemd launched an unexpected command")
}

func (env *AlloyEnvironment) logSystemdDiagnosticsOnFailure(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		t.Logf("systemctl status alloy:\n%s", env.ExecScript(`systemctl --no-pager status alloy`).Stdout)
		t.Logf("journalctl -u alloy:\n%s", env.ExecScript(`journalctl -u alloy --no-pager`).Stdout)
	})
}

func (env *AlloyEnvironment) TestDataFolderPermissions(t *testing.T) {
	// Installing should create /var/lib/alloy, assign it to the
	// alloy user and group, and set its permissions to 0770.
	res := env.Install()
	require.Equal(t, 0, res.ExitCode, "installation failed")

	res = env.ExecScript(`[ -d /var/lib/alloy/data ]`)
	require.Equal(t, 0, res.ExitCode, "Expected /var/lib/alloy/data to have been created during install")

	res = env.ExecScript(`stat -c '%a:%U:%G' /var/lib/alloy/data`)
	require.Equal(t, "770:alloy:alloy\n", res.Stdout, "wrong permissions for data folder")
	require.Equal(t, 0, res.ExitCode, "stat'ing data folder failed")
}
