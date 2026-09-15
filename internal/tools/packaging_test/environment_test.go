//go:build !nonetwork && !nodocker && packaging

package packaging_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

const (
	// systemdBootTimeout is how long to wait for systemd to finish booting in a
	// freshly started container.
	systemdBootTimeout = 60 * time.Second

	// serviceStartTimeout is how long to wait for the alloy service to become
	// active after being started or restarted.
	serviceStartTimeout = 30 * time.Second

	// pollInterval is how often to re-check a systemd state that's being waited on.
	pollInterval = 500 * time.Millisecond

	// serviceSettleTime is how long to let the service run before checking that
	// it stayed up
	serviceSettleTime = 3 * time.Second
)

type Environment struct {
	Install    func() ExecResult
	Uninstall  func() ExecResult
	ExecScript func(string) ExecResult
}

type ExecResult struct {
	Stdout, Stderr string
	ExitCode       int
}

// RPMEnvironment creates an Environment to install an RPM against.
func RPMEnvironment(t *testing.T, packageName string, pool *dockertest.Pool) Environment {
	t.Helper()

	container := environmentContainer(
		t,
		pool,
		"testdata/rocky-systemd.Dockerfile",
		packageName+"-test-rocky-systemd",
		fmt.Sprintf("../../../dist/%s-0.0.0-1.%s.rpm", packageName, runtime.GOARCH),
	)

	return Environment{
		Install: func() ExecResult {
			filename := fmt.Sprintf("/tmp/%s-0.0.0-1.%s.rpm", packageName, runtime.GOARCH)
			return containerExec(t, container, "rpm", "-i", filename)
		},
		Uninstall: func() ExecResult {
			return containerExec(t, container, "rpm", "-e", packageName)
		},
		ExecScript: func(script string) ExecResult {
			return containerExec(t, container, "/bin/bash", "-c", script)
		},
	}
}

// DEBEnvironment creates an Environment to install a DEB against.
func DEBEnvironment(t *testing.T, packageName string, pool *dockertest.Pool) Environment {
	t.Helper()

	container := environmentContainer(
		t,
		pool,
		"testdata/debian-systemd.Dockerfile",
		packageName+"-test-debian-systemd",
		fmt.Sprintf("../../../dist/%s-0.0.0-1.%s.deb", packageName, runtime.GOARCH),
	)

	return Environment{
		Install: func() ExecResult {
			filename := fmt.Sprintf("/tmp/%s-0.0.0-1.%s.deb", packageName, runtime.GOARCH)
			return containerExec(t, container, "dpkg", "--force-confold", "-i", filename)
		},
		Uninstall: func() ExecResult {
			return containerExec(t, container, "dpkg", "-r", packageName)
		},
		ExecScript: func(script string) ExecResult {
			return containerExec(t, container, "/bin/bash", "-c", script)
		},
	}
}

func environmentContainer(t *testing.T, pool *dockertest.Pool, dockerfile string, name string, packagePath string) *dockertest.Resource {
	t.Helper()

	// The images run systemd as PID 1 instead of a shell, so the tests can
	// exercise the installed alloy.service
	container, err := pool.BuildAndRunWithOptions(
		dockerfile,
		&dockertest.RunOptions{
			Name:       name,
			Privileged: true,
			PortBindings: map[docker.Port][]docker.PortBinding{
				"9009/tcp": {{HostIP: "0.0.0.0", HostPort: "0"}},
			},
		},
		func(hc *docker.HostConfig) {
			// Placing the container under a systemd slice keeps systemd's cgroup
			// management working on hosts where Docker uses the systemd cgroup
			// driver, such as GitHub's runners.
			hc.CgroupParent = "docker.slice"
			hc.Tmpfs = map[string]string{
				"/run":      "rw",
				"/run/lock": "rw",
			}
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Close()
	})

	waitForSystemd(t, container)

	packageFile, err := buildTar(packagePath)
	require.NoError(t, err)
	err = pool.Client.UploadToContainer(container.Container.ID, docker.UploadToContainerOptions{
		InputStream: packageFile,
		Path:        "/tmp",
	})
	require.NoError(t, err)

	return container
}

func waitForSystemd(t *testing.T, container *dockertest.Resource) {
	t.Helper()

	var state string
	deadline := time.Now().Add(systemdBootTimeout)
	for time.Now().Before(deadline) {
		state = strings.TrimSpace(containerExec(t, container, "systemctl", "is-system-running").Stdout)
		if state == "running" || state == "degraded" {
			return
		}
		time.Sleep(pollInterval)
	}

	require.Failf(t, "systemd did not finish booting", "last reported state: %q", state)
}

func buildTar(path string) (io.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	defer w.Close()

	err = w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     filepath.Base(path),
		Size:     fi.Size(),
		ModTime:  fi.ModTime(),
		Mode:     0600,
	})
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(w, f)
	if err != nil {
		return nil, err
	}
	return &buf, err
}

func containerExec(t *testing.T, res *dockertest.Resource, cmd ...string) ExecResult {
	t.Helper()

	var stdout, stderr bytes.Buffer

	exitCode, err := res.Exec(cmd, dockertest.ExecOptions{
		StdOut: &stdout,
		StdErr: &stderr,
	})
	require.NoError(t, err)

	return ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}
