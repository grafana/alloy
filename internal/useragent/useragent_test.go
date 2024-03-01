package useragent

import (
	"testing"

	"github.com/grafana/agent/internal/build"
	"github.com/stretchr/testify/require"
)

func TestUserAgent(t *testing.T) {
	build.Version = "v1.2.3"
	tests := []struct {
		Name       string
		Mode       string
		Expected   string
		DeployMode string
		GOOS       string
		Exe        string
	}{
		{
			Name:     "basic",
			Mode:     "",
			Expected: "GrafanaAgent/v1.2.3 (static; linux; binary)",
			GOOS:     "linux",
		},
		{
			Name:     "flow",
			Mode:     "flow",
			Expected: "GrafanaAgent/v1.2.3 (flow; windows; binary)",
			GOOS:     "windows",
		},
		{
			Name:     "static",
			Mode:     "static",
			Expected: "GrafanaAgent/v1.2.3 (static; darwin; binary)",
			GOOS:     "darwin",
		},
		{
			Name: "unknown",
			Mode: "blahlahblah",
			// unknown mode, should not happen. But we will substitute 'unknown' to avoid allowing arbitrary cardinality.
			Expected: "GrafanaAgent/v1.2.3 (unknown; freebsd; binary)",
			GOOS:     "freebsd",
		},
		{
			Name:       "operator",
			Mode:       "static",
			DeployMode: "operator",
			Expected:   "GrafanaAgent/v1.2.3 (static; linux; operator)",
			GOOS:       "linux",
		},
		{
			Name:       "deb",
			Mode:       "flow",
			DeployMode: "deb",
			Expected:   "GrafanaAgent/v1.2.3 (flow; linux; deb)",
			GOOS:       "linux",
		},
		{
			Name:       "rpm",
			Mode:       "static",
			DeployMode: "rpm",
			Expected:   "GrafanaAgent/v1.2.3 (static; linux; rpm)",
			GOOS:       "linux",
		},
		{
			Name:       "docker",
			Mode:       "flow",
			DeployMode: "docker",
			Expected:   "GrafanaAgent/v1.2.3 (flow; linux; docker)",
			GOOS:       "linux",
		},
		{
			Name:       "helm",
			Mode:       "flow",
			DeployMode: "helm",
			Expected:   "GrafanaAgent/v1.2.3 (flow; linux; helm)",
			GOOS:       "linux",
		},
		{
			Name:     "brew",
			Mode:     "flow",
			Expected: "GrafanaAgent/v1.2.3 (flow; darwin; brew)",
			GOOS:     "darwin",
			Exe:      "/opt/homebrew/bin/agent",
		},
	}
	for _, tst := range tests {
		t.Run(tst.Name, func(t *testing.T) {
			if tst.Exe != "" {
				executable = func() (string, error) { return tst.Exe, nil }
			} else {
				executable = func() (string, error) { return "/agent", nil }
			}
			goos = tst.GOOS
			t.Setenv(deployModeEnv, tst.DeployMode)
			t.Setenv(modeEnv, tst.Mode)
			actual := Get()
			require.Equal(t, tst.Expected, actual)
		})
	}
}
