package discovery

import (
	"fmt"
	"slices"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

func TestDecodeMap(t *testing.T) {
	scope := vm.NewScope(map[string]interface{}{
		"foobar": 42,
	})

	input := `{ a = "5", b = "10" }`
	expected := NewTargetFromMap(map[string]string{"a": "5", "b": "10"})

	expr, err := parser.ParseExpression(input)
	require.NoError(t, err)

	eval := vm.New(expr)
	actual := Target{}
	require.NoError(t, eval.Evaluate(scope, &actual))
	require.Equal(t, expected, actual)

	// Test can use it like a map
	var seen []string
	actual.ForEachLabel(func(k string, v string) bool {
		seen = append(seen, fmt.Sprintf("%s=%s", k, v))
		return true
	})
	slices.Sort(seen)
	require.Equal(t, []string{"a=5", "b=10"}, seen)

	actual.Set("foo", "bar")
	get, ok := actual.Get("foo")
	require.True(t, ok)
	require.Equal(t, "bar", get)
	
	actual.Delete("foo")
	get, ok = actual.Get("foo")
	require.False(t, ok)
	require.Equal(t, "", get)
}

func TestConvertFromNative(t *testing.T) {
	var nativeTargets = []model.LabelSet{
		{model.LabelName("hip"): model.LabelValue("hop")},
		{model.LabelName("nae"): model.LabelValue("nae")},
	}

	nativeGroup := &targetgroup.Group{
		Targets: nativeTargets,
		Labels: model.LabelSet{
			model.LabelName("boom"): model.LabelValue("bap"),
		},
		Source: "test",
	}

	expected := []Target{
		NewTargetFromMap(map[string]string{"hip": "hop", "boom": "bap"}),
		NewTargetFromMap(map[string]string{"nae": "nae", "boom": "bap"}),
	}

	require.Equal(t, expected, toAlloyTargets(map[string]*targetgroup.Group{"test": nativeGroup}))
}
