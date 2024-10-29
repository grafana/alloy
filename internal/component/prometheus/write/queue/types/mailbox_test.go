package types

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStop(t *testing.T) {
	mb := NewMailbox[string](0, false)
	mb.Start()
	err := mb.Send(context.Background(), "first")
	require.NoError(t, err)
	wg := sync.WaitGroup{}
	wg.Add(1)
	// This should simulate sending several items.
	go func() {
		require.Eventually(t, func() bool {
			err := mb.Send(context.Background(), "second")
			if err == nil {
				return false
			}
			result := strings.Contains(err.Error(), "stopped")
			wg.Done()
			return result
		}, 5*time.Second, 1*time.Second)
	}()
	mb.Stop()
	wg.Wait()
}
