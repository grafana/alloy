package collector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithQueryTimeoutPropagatesParentCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	err := withQueryTimeout(parent, time.Minute, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	require.ErrorIs(t, err, context.Canceled)
}

func TestWithQueryTimeoutAppliesConfiguredDeadline(t *testing.T) {
	err := withQueryTimeout(context.Background(), time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
}
