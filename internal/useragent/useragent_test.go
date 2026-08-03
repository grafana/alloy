package useragent

import (
	"testing"

	"github.com/grafana/alloy/internal/build"
	"github.com/stretchr/testify/require"
)

func TestUserAgent(t *testing.T) {
	build.Version = "v1.2.3"
	runArgs := []string{"alloy", "run", "config.alloy"}
	tests := []struct {
		Name       string
		Expected   string
		DeployMode string
		GOOS       string
		Exe        string
		Args       []string
	}{
		{
			Name:     "linux",
			Args:     runArgs,
			Expected: "Alloy/v1.2.3 (linux; binary)",
			GOOS:     "linux",
		},
		{
			Name:     "windows",
			Args:     runArgs,
			Expected: "Alloy/v1.2.3 (windows; binary)",
			GOOS:     "windows",
		},
		{
			Name:     "darwin",
			Args:     runArgs,
			Expected: "Alloy/v1.2.3 (darwin; binary)",
			GOOS:     "darwin",
		},
		{
			Name:       "deb",
			Args:       runArgs,
			DeployMode: "deb",
			Expected:   "Alloy/v1.2.3 (linux; deb)",
			GOOS:       "linux",
		},
		{
			Name:       "rpm",
			Args:       runArgs,
			DeployMode: "rpm",
			Expected:   "Alloy/v1.2.3 (linux; rpm)",
			GOOS:       "linux",
		},
		{
			Name:       "docker",
			Args:       runArgs,
			DeployMode: "docker",
			Expected:   "Alloy/v1.2.3 (linux; docker)",
			GOOS:       "linux",
		},
		{
			Name:       "helm",
			Args:       runArgs,
			DeployMode: "helm",
			Expected:   "Alloy/v1.2.3 (linux; helm)",
			GOOS:       "linux",
		},
		{
			Name:     "brew",
			Args:     runArgs,
			Expected: "Alloy/v1.2.3 (darwin; brew)",
			GOOS:     "darwin",
			Exe:      "/opt/homebrew/bin/alloy",
		},
		{
			Name:       "otel engine",
			Args:       []string{"alloy", "otel"},
			DeployMode: "docker",
			Expected:   "Alloy OTel Extension/v1.2.3 (linux; docker)",
			GOOS:       "linux",
		},
		{
			Name:     "otel engine with flags",
			Args:     []string{"alloy", "otel", "--config", "config.yaml"},
			Expected: "Alloy OTel Extension/v1.2.3 (linux; binary)",
			GOOS:     "linux",
		},
		{
			Name:     "no subcommand",
			Args:     []string{"alloy"},
			Expected: "Alloy/v1.2.3 (linux; binary)",
			GOOS:     "linux",
		},
		{
			Name:     "convert subcommand",
			Args:     []string{"alloy", "convert", "config.river"},
			Expected: "Alloy/v1.2.3 (linux; binary)",
			GOOS:     "linux",
		},
		{
			Name:     "run flag value that looks like otel",
			Args:     []string{"alloy", "run", "--config.format", "otel"},
			Expected: "Alloy/v1.2.3 (linux; binary)",
			GOOS:     "linux",
		},
	}
	for _, tst := range tests {
		t.Run(tst.Name, func(t *testing.T) {
			if tst.Exe != "" {
				executable = func() (string, error) { return tst.Exe, nil }
			} else {
				executable = func() (string, error) { return "/alloy", nil }
			}
			goos = tst.GOOS
			args = tst.Args
			t.Setenv(deployModeEnv, tst.DeployMode)
			actual := Get()
			require.Equal(t, tst.Expected, actual)
		})
	}
}
