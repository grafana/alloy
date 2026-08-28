//go:build windows

package file

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// TestGlobResolverCaseInsensitive verifies that glob patterns are case-insensitive on Windows.
// An uppercase pattern SHOULD match lowercase files.
func TestGlobResolverCaseInsensitive(t *testing.T) {
	resolver := newGlobResolver(logging.NewSlogNop())

	// Use uppercase pattern - SHOULD match the lowercase .log files on Windows
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

	require.Len(t, results, 2, "Windows should be case-insensitive: *.LOG should match *.log files")
}

// TestGlobResolverCaseInsensitiveLowercase verifies that lowercase patterns also work.
func TestGlobResolverCaseInsensitiveLowercase(t *testing.T) {
	resolver := newGlobResolver(logging.NewSlogNop())

	// Use lowercase pattern - should also match
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

	require.Len(t, results, 2, "Lowercase pattern should also find the files")
}
