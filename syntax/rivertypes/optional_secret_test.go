package rivertypes_test

import (
	"testing"

	"github.com/grafana/river/rivertypes"
	"github.com/grafana/river/token/builder"
	"github.com/stretchr/testify/require"
)

func TestOptionalSecret(t *testing.T) {
	t.Run("non-sensitive conversion to string is allowed", func(t *testing.T) {
		input := rivertypes.OptionalSecret{IsSecret: false, Value: "testval"}

		var s string
		err := decodeTo(t, input, &s)
		require.NoError(t, err)
		require.Equal(t, "testval", s)
	})

	t.Run("sensitive conversion to string is disallowed", func(t *testing.T) {
		input := rivertypes.OptionalSecret{IsSecret: true, Value: "testval"}

		var s string
		err := decodeTo(t, input, &s)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "secrets may not be converted into strings")
	})

	t.Run("non-sensitive conversion to secret is allowed", func(t *testing.T) {
		input := rivertypes.OptionalSecret{IsSecret: false, Value: "testval"}

		var s rivertypes.Secret
		err := decodeTo(t, input, &s)
		require.NoError(t, err)
		require.Equal(t, rivertypes.Secret("testval"), s)
	})

	t.Run("sensitive conversion to secret is allowed", func(t *testing.T) {
		input := rivertypes.OptionalSecret{IsSecret: true, Value: "testval"}

		var s rivertypes.Secret
		err := decodeTo(t, input, &s)
		require.NoError(t, err)
		require.Equal(t, rivertypes.Secret("testval"), s)
	})

	t.Run("conversion from string is allowed", func(t *testing.T) {
		var s rivertypes.OptionalSecret
		err := decodeTo(t, string("Hello, world!"), &s)
		require.NoError(t, err)

		expect := rivertypes.OptionalSecret{
			IsSecret: false,
			Value:    "Hello, world!",
		}
		require.Equal(t, expect, s)
	})

	t.Run("conversion from secret is allowed", func(t *testing.T) {
		var s rivertypes.OptionalSecret
		err := decodeTo(t, rivertypes.Secret("Hello, world!"), &s)
		require.NoError(t, err)

		expect := rivertypes.OptionalSecret{
			IsSecret: true,
			Value:    "Hello, world!",
		}
		require.Equal(t, expect, s)
	})
}

func TestOptionalSecret_Write(t *testing.T) {
	tt := []struct {
		name   string
		value  interface{}
		expect string
	}{
		{"non-sensitive", rivertypes.OptionalSecret{Value: "foobar"}, `"foobar"`},
		{"sensitive", rivertypes.OptionalSecret{IsSecret: true, Value: "foobar"}, `(secret)`},
		{"non-sensitive pointer", &rivertypes.OptionalSecret{Value: "foobar"}, `"foobar"`},
		{"sensitive pointer", &rivertypes.OptionalSecret{IsSecret: true, Value: "foobar"}, `(secret)`},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			be := builder.NewExpr()
			be.SetValue(tc.value)
			require.Equal(t, tc.expect, string(be.Bytes()))
		})
	}
}
