package util

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitTrigger(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		wt := NewWaitTrigger()
		err := wt.Wait(time.Millisecond * 100)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("no timeout", func(t *testing.T) {
		wt := NewWaitTrigger()

		go func() {
			<-time.After(100 * time.Millisecond)
			wt.Trigger()
		}()

		err := wt.Wait(time.Second)
		require.NoError(t, err)
	})
}
