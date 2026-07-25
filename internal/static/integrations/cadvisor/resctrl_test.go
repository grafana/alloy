//go:build linux

package cadvisor

import (
	"testing"

	"github.com/google/cadvisor/resctrl"
	"github.com/stretchr/testify/require"
)

// TestResctrlManagerIsAlwaysUsable guards against the startup panic reported in
// https://github.com/grafana/alloy/issues/5838. cAdvisor's manager calls resctrl.NewManager once at
// startup, keeps whatever it returns, and dereferences it for every container it discovers while
// the resctrl metric kind is included, without checking for nil. resctrl.NewManager only returns a
// manager if a plugin is registered, so the integration has to register one.
func TestResctrlManagerIsAlwaysUsable(t *testing.T) {
	mgr, err := resctrl.NewManager(0, "GenuineIntel", true)
	require.NotNil(t, mgr, "cAdvisor dereferences the resctrl manager without a nil check, so it must never be nil")

	if err == nil {
		// resctrl is supported on this machine, so the manager is the real one. Don't set up
		// monitoring groups against the host from a unit test.
		return
	}

	// resctrl isn't supported here, which is the case that used to panic. The fallback manager must
	// still hand out a usable collector.
	collector, err := mgr.GetCollector("/alloy-test", func() ([]string, error) { return nil, nil }, 1)
	require.NoError(t, err)
	require.NotNil(t, collector)
}
