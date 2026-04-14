package harness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func MustComponent[T any](t *testing.T, a *Alloy, id string) T {
	t.Helper()

	c, err := a.Component(id)
	require.NoError(t, err)

	typed, ok := c.(T)
	require.Truef(t, ok, "component %q has type %T, want %T", id, c, *new(T))

	return typed
}
