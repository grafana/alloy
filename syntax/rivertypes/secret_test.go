package rivertypes_test

import (
	"testing"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/rivertypes"
	"github.com/grafana/river/vm"
	"github.com/stretchr/testify/require"
)

func TestSecret(t *testing.T) {
	t.Run("strings can be converted to secret", func(t *testing.T) {
		var s rivertypes.Secret
		err := decodeTo(t, string("Hello, world!"), &s)
		require.NoError(t, err)
		require.Equal(t, rivertypes.Secret("Hello, world!"), s)
	})

	t.Run("secrets cannot be converted to strings", func(t *testing.T) {
		var s string
		err := decodeTo(t, rivertypes.Secret("Hello, world!"), &s)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "secrets may not be converted into strings")
	})

	t.Run("secrets can be passed to secrets", func(t *testing.T) {
		var s rivertypes.Secret
		err := decodeTo(t, rivertypes.Secret("Hello, world!"), &s)
		require.NoError(t, err)
		require.Equal(t, rivertypes.Secret("Hello, world!"), s)
	})
}

func decodeTo(t *testing.T, input interface{}, target interface{}) error {
	t.Helper()

	expr, err := parser.ParseExpression("val")
	require.NoError(t, err)

	eval := vm.New(expr)
	return eval.Evaluate(&vm.Scope{
		Variables: map[string]interface{}{
			"val": input,
		},
	}, target)
}
