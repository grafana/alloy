//go:build !nonetwork && !nodocker && packaging
// +build !nonetwork,!nodocker,packaging

package packaging_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

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

		// TODO: a test to verify that the systemd service works would be nice, but not
		// required.
		//
		// An implementation of the test would have to consider what host platforms it
		// works on; bind mounting /sys/fs/cgroup and using the host systemd wouldn't
		// work on macOS or Windows.
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

	res = env.Install()
	require.Equal(t, 0, res.ExitCode, "installation failed")

	res = env.ExecScript(`cat /etc/alloy/config.alloy`)
	require.Equal(t, "keepalive", res.Stdout, "Expected existing file to not be overridden")
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
