package pyroscope

import (
	"testing"

	"github.com/grafana/alloy/internal/build"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestLabelsWithScopeAddsVersion(t *testing.T) {
	oldVersion := build.Version
	t.Cleanup(func() {
		build.Version = oldVersion
	})
	build.Version = "v1.2.3"

	lbs := LabelsWithScope(labels.EmptyLabels(), ScopeNameScrape)

	require.Equal(t, ScopeNameScrape, lbs.Get(LabelOtelScopeName))
	require.Equal(t, "v1.2.3", lbs.Get(LabelOtelScopeVersion))
}

func TestLabelsWithScopeOmitsFallbackVersion(t *testing.T) {
	oldVersion := build.Version
	t.Cleanup(func() {
		build.Version = oldVersion
	})
	build.Version = "v0.0.0"

	lbs := LabelsWithScope(labels.EmptyLabels(), ScopeNameScrape)

	require.Equal(t, ScopeNameScrape, lbs.Get(LabelOtelScopeName))
	require.Empty(t, lbs.Get(LabelOtelScopeVersion))
}

func TestLabelsWithScopeDoesNotOverrideExistingScopeLabels(t *testing.T) {
	oldVersion := build.Version
	t.Cleanup(func() {
		build.Version = oldVersion
	})
	build.Version = "v1.2.3"

	lbs := LabelsWithScope(labels.FromStrings(
		LabelOtelScopeName, "user-scope",
		LabelOtelScopeVersion, "user-version",
	), ScopeNameScrape)

	require.Equal(t, "user-scope", lbs.Get(LabelOtelScopeName))
	require.Equal(t, "user-version", lbs.Get(LabelOtelScopeVersion))
}

func TestLabelsWithScopeDoesNotRemoveExistingVersionWhenAlloyVersionUnavailable(t *testing.T) {
	oldVersion := build.Version
	t.Cleanup(func() {
		build.Version = oldVersion
	})
	build.Version = "v0.0.0"

	lbs := LabelsWithScope(labels.FromStrings(
		LabelOtelScopeName, "user-scope",
		LabelOtelScopeVersion, "user-version",
	), ScopeNameScrape)

	require.Equal(t, "user-scope", lbs.Get(LabelOtelScopeName))
	require.Equal(t, "user-version", lbs.Get(LabelOtelScopeVersion))
}

func TestLabelsWithScopeDoesNotAddAlloyVersionForUserScope(t *testing.T) {
	oldVersion := build.Version
	t.Cleanup(func() {
		build.Version = oldVersion
	})
	build.Version = "v1.2.3"

	lbs := LabelsWithScope(labels.FromStrings(LabelOtelScopeName, "user-scope"), ScopeNameScrape)

	require.Equal(t, "user-scope", lbs.Get(LabelOtelScopeName))
	require.Empty(t, lbs.Get(LabelOtelScopeVersion))
}
