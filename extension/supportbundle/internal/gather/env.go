package gather

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// defaultEnvVars is the built-in allowlist of environment variables. It matches
// the Alloy engine support bundle. The list excludes credential-bearing
// variables on purpose.
var defaultEnvVars = []string{
	"AUTOMEMLIMIT",
	"GODEBUG",
	"GOGC",
	"GOMAXPROCS",
	"GOMEMLIMIT",
	"HOSTNAME",
	"HTTP_PROXY",
	"http_proxy",
	"HTTPS_PROXY",
	"https_proxy",
	"NO_PROXY",
	"no_proxy",
	"PPROF_BLOCK_PROFILING_RATE",
	"PPROF_MUTEX_PROFILING_PERCENT",
}

// Environment writes an allowlist of environment variables to the bundle.
// The allowlist is fixed at config time. A request cannot extend it, so a
// caller cannot read arbitrary environment variables.
type Environment struct {
	// Extra holds extra variable names from the config.
	Extra []string
}

func (Environment) Name() string { return "environment" }

func (g Environment) Gather(_ context.Context, _ Options) ([]File, error) {
	var b strings.Builder
	for _, name := range mergeEnvNames(defaultEnvVars, g.Extra) {
		if val, ok := os.LookupEnv(name); ok {
			fmt.Fprintf(&b, "%s=%s\n", name, val)
		}
	}

	if b.Len() == 0 {
		return nil, nil
	}
	return []File{{Path: "environment.txt", Content: []byte(b.String())}}, nil
}

// mergeEnvNames returns the sorted, de-duplicated union of the name lists.
func mergeEnvNames(base, extra []string) []string {
	seen := make(map[string]struct{}, len(base)+len(extra))
	var names []string
	for _, list := range [][]string{base, extra} {
		for _, name := range list {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
