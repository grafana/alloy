//go:build !nonetwork && !nodocker && !race && packaging
// +build !nonetwork,!nodocker,!race,packaging

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

// TestFlowLinuxPackages runs the entire test suite for the Linux packages.
func TestFlowLinuxPackages(t *testing.T) {
	packageName := "grafana-agent-flow"

	fmt.Println("Building packages (this may take a while...)")
	buildFlowPackages(t)

	dockerPool, err := dockertest.NewPool("")
	require.NoError(t, err)

	tt := []struct {
		name string
		f    func(*FlowEnvironment, *testing.T)
	}{
		{"install package", (*FlowEnvironment).TestInstall},
		{"ensure existing config doesn't get overridden", (*FlowEnvironment).TestConfigPersistence},
		{"test data folder permissions", (*FlowEnvironment).TestDataFolderPermissions},

		// TODO: a test to verify that the systemd service works would be nice, but not
		// required.
		//
		// An implementation of the test would have to consider what host platforms it
		// works on; bind mounting /sys/fs/cgroup and using the host systemd wouldn't
		// work on macOS or Windows.
	}

	for _, tc := range tt {
		t.Run(tc.name+"/rpm", func(t *testing.T) {
			env := &FlowEnvironment{RPMEnvironment(t, packageName, dockerPool)}
			tc.f(env, t)
		})
		t.Run(tc.name+"/deb", func(t *testing.T) {
			env := &FlowEnvironment{DEBEnvironment(t, packageName, dockerPool)}
			tc.f(env, t)
		})
	}
}

func buildFlowPackages(t *testing.T) {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	root, err := filepath.Abs(filepath.Join(wd, "../../.."))
	require.NoError(t, err)

	cmd := exec.Command("make", fmt.Sprintf("dist-agent-flow-packages-%s", runtime.GOARCH))
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

type FlowEnvironment struct{ Environment }

func (env *FlowEnvironment) TestInstall(t *testing.T) {
	res := env.Install()
	require.Equal(t, 0, res.ExitCode, "installing failed")

	res = env.ExecScript(`[ -f /usr/bin/grafana-agent-flow ]`)
	require.Equal(t, 0, res.ExitCode, "expected grafana-agent-flow to be installed")
	res = env.ExecScript(`[ -f /etc/grafana-agent-flow.river ]`)
	require.Equal(t, 0, res.ExitCode, "expected grafana agent configuration file to exist")

	res = env.Uninstall()
	require.Equal(t, 0, res.ExitCode, "uninstalling failed")

	res = env.ExecScript(`[ -f /usr/bin/grafana-agent-flow ]`)
	require.Equal(t, 1, res.ExitCode, "expected grafana-agent-flow to be uninstalled")
	// NOTE(rfratto): we don't check for what happens to the config file here,
	// since the behavior is inconsistent: rpm uninstalls it, but deb doesn't.
}

func (env *FlowEnvironment) TestConfigPersistence(t *testing.T) {
	res := env.ExecScript(`echo -n "keepalive" > /etc/grafana-agent.river`)
	require.Equal(t, 0, res.ExitCode, "failed to write config file")

	res = env.Install()
	require.Equal(t, 0, res.ExitCode, "installation failed")

	res = env.ExecScript(`cat /etc/grafana-agent.river`)
	require.Equal(t, "keepalive", res.Stdout, "Expected existing file to not be overridden")
}

func (env *FlowEnvironment) TestDataFolderPermissions(t *testing.T) {
	// Installing should create /var/lib/grafana-agent, assign it to the
	// grafana-agent user and group, and set its permissions to 0770.
	res := env.Install()
	require.Equal(t, 0, res.ExitCode, "installation failed")

	res = env.ExecScript(`[ -d /var/lib/grafana-agent-flow ]`)
	require.Equal(t, 0, res.ExitCode, "Expected /var/lib/grafana-agent-flow to have been created during install")

	res = env.ExecScript(`stat -c '%a:%U:%G' /var/lib/grafana-agent-flow`)
	require.Equal(t, "770:grafana-agent-flow:grafana-agent\n", res.Stdout, "wrong permissions for data folder")
	require.Equal(t, 0, res.ExitCode, "stat'ing data folder failed")
}
