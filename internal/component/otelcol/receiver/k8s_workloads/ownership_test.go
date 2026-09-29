package k8s_workloads

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOwnershipSettling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Component{}
		done := make(chan error, 1)
		go func() { done <- c.waitForClusterStability(t.Context()) }()
		synctest.Wait()
		time.Sleep(20 * time.Second)
		c.mu.Lock()
		c.clusterLastChanged = time.Now()
		c.mu.Unlock()
		time.Sleep(20 * time.Second)
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("started before membership settled")
		default:
		}
		time.Sleep(10 * time.Second)
		require.NoError(t, <-done)
	})
}
func TestOwnershipSettlingCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Component{}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- c.waitForClusterStability(ctx) }()
		synctest.Wait()
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
	})
}
func TestOwnershipSettlingBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Component{}
		done := make(chan error, 1)
		go func() { done <- c.waitForClusterStability(t.Context()) }()
		synctest.Wait()
		for range 8 {
			time.Sleep(10 * time.Second)
			c.mu.Lock()
			c.clusterLastChanged = time.Now()
			c.mu.Unlock()
		}
		time.Sleep(10 * time.Second)
		require.NoError(t, <-done)
	})
}
