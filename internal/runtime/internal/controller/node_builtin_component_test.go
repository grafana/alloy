package controller

import (
	"path/filepath"
	"testing"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/stretchr/testify/require"
)

func TestGlobalID(t *testing.T) {
	mo := getManagedOptions(ComponentGlobals{
		DataPath:     "/data/",
		MinStability: featuregate.StabilityPublicPreview,
		ControllerID: "module.file",
		NewModuleController: func(opts ModuleControllerOpts) ModuleController {
			return nil
		},
	}, &BuiltinComponentNode{
		nodeID:   "local.id",
		globalID: "module.file/local.id",
	})
	require.Equal(t, "/data/module.file/local.id", filepath.ToSlash(mo.DataPath))
}

func TestLocalID(t *testing.T) {
	mo := getManagedOptions(ComponentGlobals{
		DataPath:     "/data/",
		MinStability: featuregate.StabilityPublicPreview,
		ControllerID: "",
		NewModuleController: func(opts ModuleControllerOpts) ModuleController {
			return nil
		},
	}, &BuiltinComponentNode{
		nodeID:   "local.id",
		globalID: "local.id",
	})
	require.Equal(t, "/data/local.id", filepath.ToSlash(mo.DataPath))
}

func TestSplitPath(t *testing.T) {
	var testcases = []struct {
		input string
		path  string
		id    string
	}{
		{"", "/", ""},
		{"remotecfg", "/", "remotecfg"},
		{"prometheus.remote_write", "/", "prometheus.remote_write"},
		{"custom_component.default/prometheus.remote_write", "/custom_component.default", "prometheus.remote_write"},

		{"local.file.default", "/", "local.file.default"},
		{"a_namespace.a.default/local.file.default", "/a_namespace.a.default", "local.file.default"},
		{"a_namespace.a.default/b_namespace.b.default/local.file.default", "/a_namespace.a.default/b_namespace.b.default", "local.file.default"},

		{"a_namespace.a.default/b_namespace.b.default/c_namespace.c.default", "/a_namespace.a.default/b_namespace.b.default", "c_namespace.c.default"},
	}

	for _, tt := range testcases {
		path, id := splitPath(tt.input)
		require.Equal(t, tt.path, path)
		require.Equal(t, tt.id, id)
	}
}
