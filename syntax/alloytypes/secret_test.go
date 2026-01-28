package alloytypes_test

import (
	"testing"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
	"github.com/stretchr/testify/require"
)

func TestSecret(t *testing.T) {
	t.Run("strings can be converted to secret", func(t *testing.T) {
		var s alloytypes.Secret
		err := decodeTo(t, string("Hello, world!"), &s)
		require.NoError(t, err)
		require.Equal(t, alloytypes.Secret("Hello, world!"), s)
	})

	t.Run("secrets cannot be converted to strings", func(t *testing.T) {
		var s string
		err := decodeTo(t, alloytypes.Secret("Hello, world!"), &s)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "secrets may not be converted into strings")
	})

	t.Run("secrets can be passed to secrets", func(t *testing.T) {
		var s alloytypes.Secret
		err := decodeTo(t, alloytypes.Secret("Hello, world!"), &s)
		require.NoError(t, err)
		require.Equal(t, alloytypes.Secret("Hello, world!"), s)
	})
}

func decodeTo(t *testing.T, input any, target any) error {
	t.Helper()

	expr, err := parser.ParseExpression("val")
	require.NoError(t, err)

	eval := vm.New(expr)
	return eval.Evaluate(vm.NewScope(map[string]any{
		"val": input,
	}), target)
}
