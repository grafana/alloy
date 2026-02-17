package file_match

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/local/file_match/testutil"
	"github.com/grafana/alloy/internal/util"
)

// testCreateComponent creates a component with the given paths and excluded patterns.
func testCreateComponent(t *testing.T, dir string, paths []string, excluded []string) *Component {
	return testCreateComponentWithLabels(t, dir, paths, excluded, nil)
}

// testCreateComponentWithLabels creates a component with the given paths, excluded patterns, and labels.
func testCreateComponentWithLabels(t *testing.T, dir string, paths []string, excluded []string, labels map[string]string) *Component {
	t.Helper()
	tPaths := testutil.MakeTargets(paths, excluded, labels)
	c, err := New(component.Options{
		ID:       "test",
		Logger:   util.TestAlloyLogger(t),
		DataPath: dir,
		OnStateChange: func(e component.Exports) {
		},
		Registerer: prometheus.DefaultRegisterer,
		Tracer:     nil,
	}, Arguments{
		PathTargets: tPaths,
		SyncPeriod:  1 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	return c
}
