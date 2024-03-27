package useragent

import (
	"testing"

	"github.com/grafana/alloy/internal/build"
	"github.com/stretchr/testify/require"
)

func TestUserAgent(t *testing.T) {
	build.Version = "v1.2.3"
	tests := []struct {
		Name       string
		Expected   string
		DeployMode string
		GOOS       string
		Exe        string
	}{
		{
			Name:     "linux",
			Expected: "Alloy/v1.2.3 (linux; binary)",
			GOOS:     "linux",
		},
		{
			Name:     "windows",
			Expected: "Alloy/v1.2.3 (windows; binary)",
			GOOS:     "windows",
		},
		{
			Name:     "darwin",
			Expected: "Alloy/v1.2.3 (darwin; binary)",
			GOOS:     "darwin",
		},
		{
			Name:       "deb",
			DeployMode: "deb",
			Expected:   "Alloy/v1.2.3 (linux; deb)",
			GOOS:       "linux",
		},
		{
			Name:       "rpm",
			DeployMode: "rpm",
			Expected:   "Alloy/v1.2.3 (linux; rpm)",
			GOOS:       "linux",
		},
		{
			Name:       "docker",
			DeployMode: "docker",
			Expected:   "Alloy/v1.2.3 (linux; docker)",
			GOOS:       "linux",
		},
		{
			Name:       "helm",
			DeployMode: "helm",
			Expected:   "Alloy/v1.2.3 (linux; helm)",
			GOOS:       "linux",
		},
		{
			Name:     "brew",
			Expected: "Alloy/v1.2.3 (darwin; brew)",
			GOOS:     "darwin",
			Exe:      "/opt/homebrew/bin/alloy",
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
			t.Setenv(deployModeEnv, tst.DeployMode)
			actual := Get()
			require.Equal(t, tst.Expected, actual)
		})
	}
}
