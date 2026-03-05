//go:build !windows

package file

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
)

// TestGlobResolverCaseSensitive verifies that glob patterns are case-sensitive on Unix.
// An uppercase pattern should NOT match lowercase files.
func TestGlobResolverCaseSensitive(t *testing.T) {
	resolver := newGlobResolver(log.NewNopLogger())

	// Use uppercase pattern - should NOT match the lowercase .log files on Unix
	targets := []discovery.Target{
		discovery.NewTargetFromLabelSet(model.LabelSet{
			"__path__": "./testdata/*.LOG",
			"label":    "test",
		}),
	}

	var results []resolvedTarget
	for target := range resolver.Resolve(targets) {
		results = append(results, target)
	}

	require.Len(t, results, 0, "Unix should be case-sensitive: *.LOG should not match *.log files")
}

// TestGlobResolverCaseSensitiveMatch verifies that matching case works correctly.
func TestGlobResolverCaseSensitiveMatch(t *testing.T) {
	resolver := newGlobResolver(log.NewNopLogger())

	// Use lowercase pattern - should match the lowercase .log files
	targets := []discovery.Target{
		discovery.NewTargetFromLabelSet(model.LabelSet{
			"__path__": "./testdata/*.log",
			"label":    "test",
		}),
	}

	var results []resolvedTarget
	for target := range resolver.Resolve(targets) {
		results = append(results, target)
	}

	require.Len(t, results, 2, "Pattern with matching case should find the files")
}
