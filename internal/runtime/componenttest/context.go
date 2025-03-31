package componenttest

import (
	"context"
	"testing"
)

// TestContext returns a context which cancels itself when t finishes.
func TestContext(t testing.TB) context.Context {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return ctx
}
