package gather

import (
	"context"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildInfoGather(t *testing.T) {
	files, err := BuildInfo{}.Gather(context.Background(), Options{})
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "build-info.txt", files[0].Path)

	content := string(files[0].Content)
	require.Contains(t, content, "go_version:")
	require.Contains(t, content, "main:")
	require.Contains(t, content, "settings:")
}

// TestFormatBuildInfo checks the dependency and replace formatting with a fixed
// build info, which a test binary's real build info does not always provide.
func TestFormatBuildInfo(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.99.0",
		Path:      "example.com/main",
		Main:      debug.Module{Path: "example.com/main", Version: "(devel)"},
		Settings:  []debug.BuildSetting{{Key: "GOARCH", Value: "arm64"}},
		Deps: []*debug.Module{
			{Path: "example.com/plain", Version: "v1.2.3"},
			{
				Path:    "example.com/replaced",
				Version: "v0.1.0",
				Replace: &debug.Module{Path: "../local", Version: "v0.0.0"},
			},
		},
	}

	out := formatBuildInfo(info)
	require.Contains(t, out, "go_version: go1.99.0")
	require.Contains(t, out, "main: example.com/main (devel)")
	require.Contains(t, out, "  GOARCH=arm64")
	require.Contains(t, out, "  example.com/plain v1.2.3")
	require.Contains(t, out, "  example.com/replaced v0.1.0 => ../local v0.0.0")
}

func TestRuntimeFlagsGather(t *testing.T) {
	files, err := RuntimeFlags{}.Gather(context.Background(), Options{})
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "runtime-flags.txt", files[0].Path)
	require.Equal(t, strings.Join(os.Args, "\n")+"\n", string(files[0].Content))
}
