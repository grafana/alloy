package gather

import (
	"context"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
)

// BuildInfo writes the Go build information to the bundle. It records the main
// module, the build settings (including the VCS revision), and every dependency
// with its version. Support uses this to know exactly which component versions
// are compiled into the distribution.
type BuildInfo struct{}

func (BuildInfo) Name() string { return "build-info" }

func (BuildInfo) Gather(_ context.Context, _ Options) ([]File, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		// Build info is absent only for a binary built without module support.
		return nil, nil
	}
	return []File{{Path: "build-info.txt", Content: []byte(formatBuildInfo(info))}}, nil
}

// formatBuildInfo renders the build info as text: the main module, the sorted
// build settings, and the sorted dependency list.
func formatBuildInfo(info *debug.BuildInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "go_version: %s\n", info.GoVersion)
	fmt.Fprintf(&b, "path: %s\n", info.Path)
	fmt.Fprintf(&b, "main: %s %s\n", info.Main.Path, info.Main.Version)

	if len(info.Settings) > 0 {
		settings := append([]debug.BuildSetting(nil), info.Settings...)
		sort.Slice(settings, func(i, j int) bool { return settings[i].Key < settings[j].Key })
		b.WriteString("\nsettings:\n")
		for _, s := range settings {
			fmt.Fprintf(&b, "  %s=%s\n", s.Key, s.Value)
		}
	}

	if len(info.Deps) > 0 {
		deps := append([]*debug.Module(nil), info.Deps...)
		sort.Slice(deps, func(i, j int) bool { return deps[i].Path < deps[j].Path })
		b.WriteString("\ndependencies:\n")
		for _, d := range deps {
			// A replace directive redirects the module that is actually built.
			if d.Replace != nil {
				fmt.Fprintf(&b, "  %s %s => %s %s\n", d.Path, d.Version, d.Replace.Path, d.Replace.Version)
				continue
			}
			fmt.Fprintf(&b, "  %s %s\n", d.Path, d.Version)
		}
	}

	return b.String()
}
