// Package testutil provides test utilities for the file_match component.
package testutil

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
)

// WriteFile creates a test file with the given name in the specified directory.
func WriteFile(t *testing.T, dir string, name string) {
	t.Helper()
	err := os.WriteFile(path.Join(dir, name), []byte("test content"), 0664)
	require.NoError(t, err)
}

// ContainsPath checks if any target's __path__ contains the given substring.
func ContainsPath(sources []discovery.Target, match string) bool {
	for _, s := range sources {
		p, _ := s.Get("__path__")
		if strings.Contains(p, match) {
			return true
		}
	}
	return false
}

// MakeTargets creates discovery targets from paths and excluded patterns.
func MakeTargets(paths []string, excluded []string, labels map[string]string) []discovery.Target {
	tPaths := make([]discovery.Target, 0)
	for i, p := range paths {
		tb := discovery.NewTargetBuilder()
		tb.Set("__path__", p)
		for k, v := range labels {
			tb.Set(k, v)
		}
		if i < len(excluded) {
			tb.Set("__path_exclude__", excluded[i])
		}
		tPaths = append(tPaths, tb.Target())
	}
	return tPaths
}
