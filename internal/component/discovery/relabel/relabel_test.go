package relabel_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/discovery/relabel"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/syntax"
)

func TestRelabelConfigApplication(t *testing.T) {
	alloyArguments := `
targets = [
	{ "__meta_foo" = "foo", "__meta_bar" = "bar", "__address__" = "localhost", "instance" = "one",   "app" = "backend",  "__tmp_a" = "tmp" },
	{ "__meta_foo" = "foo", "__meta_bar" = "bar", "__address__" = "localhost", "instance" = "two",   "app" = "db",       "__tmp_b" = "tmp" },
	{ "__meta_baz" = "baz", "__meta_qux" = "qux", "__address__" = "localhost", "instance" = "three", "app" = "frontend", "__tmp_c" = "tmp" },
]

rule {
	source_labels = ["__address__", "instance"]
	separator     = "/"
	target_label  = "destination"
	action        = "replace"
}

rule {
	source_labels = ["app"]
	action        = "drop"
	regex         = "frontend"
}

rule {
	source_labels = ["app"]
	action        = "keep"
	regex         = "backend"
}

rule {
	source_labels = ["instance"]
	target_label  = "name"
}

rule {
	action      = "labelmap"
	regex       = "__meta_(.*)"
	replacement = "meta_$1"
}

rule {
	action = "labeldrop"
	regex  = "__meta(.*)|__tmp(.*)|instance"
}
`
	expectedOutput := []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{"__address__": "localhost", "app": "backend", "destination": "localhost/one", "meta_bar": "bar", "meta_foo": "foo", "name": "one"}),
	}

	var args relabel.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(alloyArguments), &args))

	tc, err := componenttest.NewControllerFromID(nil, "discovery.relabel")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()

	require.NoError(t, tc.WaitExports(time.Second))
	require.Equal(t, expectedOutput, tc.Exports().(relabel.Exports).Output)
	require.NotNil(t, tc.Exports().(relabel.Exports).Rules)
}

func TestRuleGetter(t *testing.T) {
	originalCfg := `
targets = []

rule {
	action        = "keep"
	source_labels = ["__name__"]
	regex         = "up"
}`
	var args relabel.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(originalCfg), &args))

	tc, err := componenttest.NewControllerFromID(nil, "discovery.relabel")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()

	require.NoError(t, tc.WaitExports(time.Second))

	// Use the getter to retrieve the original relabeling rules.
	exports := tc.Exports().(relabel.Exports)
	gotOriginal := exports.Rules

	// Update the component with new relabeling rules and retrieve them.
	updatedCfg := `
targets = []

rule {
	action        = "drop"
	source_labels = ["__name__"]
	regex         = "up"
}`
	require.NoError(t, syntax.Unmarshal([]byte(updatedCfg), &args))

	require.NoError(t, tc.Update(args))
	exports = tc.Exports().(relabel.Exports)
	gotUpdated := exports.Rules

	require.NotEqual(t, gotOriginal, gotUpdated)
	require.Len(t, gotOriginal, 1)
	require.Len(t, gotUpdated, 1)

	require.Equal(t, gotOriginal[0].Action, alloy_relabel.Keep)
	require.Equal(t, gotUpdated[0].Action, alloy_relabel.Drop)
	require.Equal(t, gotUpdated[0].SourceLabels, gotOriginal[0].SourceLabels)
	require.Equal(t, gotUpdated[0].Regex, gotOriginal[0].Regex)
}
