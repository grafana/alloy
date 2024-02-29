package http

import (
	"testing"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_splitURLPath(t *testing.T) {
	host := &fakeServiceHost{
		components: map[component.ID]struct{}{
			component.ParseID("prometheus.exporter.unix"):                                                   {},
			component.ParseID("module.string.example/prometheus.exporter.mysql.example"):                    {},
			component.ParseID("module.string.example/module.git.example/prometheus.exporter.mysql.example"): {},
		},
	}

	tt := []struct {
		testPath   string
		expectID   component.ID
		expectPath string
	}{
		// Root module component
		{
			testPath:   "/prometheus.exporter.unix/metrics",
			expectID:   component.ID{LocalID: "prometheus.exporter.unix"},
			expectPath: "/metrics",
		},
		// Trailing slash
		{
			testPath:   "/prometheus.exporter.unix/metrics/",
			expectID:   component.ID{LocalID: "prometheus.exporter.unix"},
			expectPath: "/metrics/",
		},
		// Component in module
		{
			testPath:   "/module.string.example/prometheus.exporter.mysql.example/metrics",
			expectID:   component.ID{ModuleID: "module.string.example", LocalID: "prometheus.exporter.mysql.example"},
			expectPath: "/metrics",
		},
		// Component in nested module
		{
			testPath:   "/module.string.example/module.git.example/prometheus.exporter.mysql.example/metrics",
			expectID:   component.ID{ModuleID: "module.string.example/module.git.example", LocalID: "prometheus.exporter.mysql.example"},
			expectPath: "/metrics",
		},
		// Path with multiple elements
		{
			testPath:   "/prometheus.exporter.unix/some/path/from/component",
			expectID:   component.ID{LocalID: "prometheus.exporter.unix"},
			expectPath: "/some/path/from/component",
		},
		// Empty path
		{
			testPath:   "/prometheus.exporter.unix",
			expectID:   component.ID{LocalID: "prometheus.exporter.unix"},
			expectPath: "/",
		},
		// Empty path with trailing slash
		{
			testPath:   "/prometheus.exporter.unix/",
			expectID:   component.ID{LocalID: "prometheus.exporter.unix"},
			expectPath: "/",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testPath, func(t *testing.T) {
			id, remain, err := splitURLPath(host, tc.testPath)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectID, id)
			assert.Equal(t, tc.expectPath, remain)
		})
	}
}

type fakeServiceHost struct {
	service.Host
	components map[component.ID]struct{}
}

func (h *fakeServiceHost) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	_, exist := h.components[id]
	if exist {
		return &component.Info{ID: id}, nil
	}

	return nil, component.ErrComponentNotFound
}

func Test_reversePathIterator(t *testing.T) {
	path := "hello/world/this/is/a/split/path"

	type pair struct{ before, after string }

	var actual []pair
	it := newReversePathIterator(path)
	for it.Next() {
		before, after := it.Value()
		actual = append(actual, pair{before, after})
	}

	expect := []pair{
		{"hello/world/this/is/a/split/path", ""},
		{"hello/world/this/is/a/split", "/path"},
		{"hello/world/this/is/a", "/split/path"},
		{"hello/world/this/is", "/a/split/path"},
		{"hello/world/this", "/is/a/split/path"},
		{"hello/world", "/this/is/a/split/path"},
		{"hello", "/world/this/is/a/split/path"},
	}

	require.Equal(t, expect, actual)
}
