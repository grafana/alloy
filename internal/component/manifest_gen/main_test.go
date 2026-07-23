package main

import (
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"

	"github.com/stretchr/testify/require"
)

func TestBuildManifest_SortsAndMapsStability(t *testing.T) {
	regs := []component.Registration{
		{Name: "zzz.ga", Stability: featuregate.StabilityGenerallyAvailable},
		{Name: "aaa.experimental", Stability: featuregate.StabilityExperimental},
		{Name: "mmm.preview", Stability: featuregate.StabilityPublicPreview},
		{Name: "ccc.community", Community: true},
	}

	m, err := buildManifest(regs)
	require.NoError(t, err)
	require.Equal(t, []entry{
		{Name: "aaa.experimental", Stability: "experimental"},
		{Name: "ccc.community", Community: true},
		{Name: "mmm.preview", Stability: "public-preview"},
		{Name: "zzz.ga", Stability: "generally-available"},
	}, m.Components)
}

func TestBuildManifest_ErrorsOnUndefinedStability(t *testing.T) {
	regs := []component.Registration{
		{Name: "bad.component", Stability: featuregate.StabilityUndefined},
	}

	_, err := buildManifest(regs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad.component")
}

func TestRenderManifest_HeaderAndYAML(t *testing.T) {
	m := manifest{Components: []entry{
		{Name: "aaa.experimental", Stability: "experimental"},
		{Name: "ccc.community", Community: true},
		{Name: "zzz.ga", Stability: "generally-available"},
	}}

	out, err := renderManifest(m)
	require.NoError(t, err)

	got := string(out)
	require.True(t, strings.HasPrefix(got, "# This file is generated. DO NOT EDIT."), "missing header, got:\n%s", got)
	require.Contains(t, got, "components:\n")
	require.Contains(t, got, "  - name: aaa.experimental\n    stability: experimental\n")
	require.Contains(t, got, "  - name: ccc.community\n    community: true\n")
	require.Contains(t, got, "  - name: zzz.ga\n    stability: generally-available\n")
}
