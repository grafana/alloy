package bridge

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax/alloytypes"
)

type normalizeRule struct {
	Match string `mapstructure:"match"`
}

type normalizeSquashed struct {
	Rules []normalizeRule `mapstructure:"rules"`
}

type normalizeConfig struct {
	normalizeSquashed `mapstructure:",squash"`

	Token  string                   `mapstructure:"token"`
	ByName map[string]normalizeRule `mapstructure:"by_name"`
}

func TestNormalize(t *testing.T) {
	in := map[string]any{
		// A single block can't be told apart from a list with one block.
		"rules": map[string]any{"match": "a"},
		// Named string types such as secrets become plain strings.
		"token":   alloytypes.Secret("s3cret"),
		"by_name": map[string]any{"x": map[string]any{"match": "b"}},
		// Unknown keys are left for confmap to report.
		"unknown": 1,
	}

	out, err := normalize(in, reflect.TypeFor[normalizeConfig]())
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"rules":   []any{map[string]any{"match": "a"}},
		"token":   "s3cret",
		"by_name": map[string]any{"x": map[string]any{"match": "b"}},
		"unknown": 1,
	}, out)
}
