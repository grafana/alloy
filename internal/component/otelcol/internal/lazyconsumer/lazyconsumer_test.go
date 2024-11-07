package lazyconsumer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_PauseAndResume(t *testing.T) {
	c := New(context.Background())
	require.False(t, c.IsPaused())
	c.Pause()
	require.True(t, c.IsPaused())
	c.Resume()
	require.False(t, c.IsPaused())
}

func Test_NewPaused(t *testing.T) {
	c := NewPaused(context.Background())
	require.True(t, c.IsPaused())
	c.Resume()
	require.False(t, c.IsPaused())
}

func Test_PauseResume_MultipleCalls(t *testing.T) {
	c := New(context.Background())
	require.False(t, c.IsPaused())
	c.Pause()
	c.Pause()
	c.Pause()
	require.True(t, c.IsPaused())
	c.Resume()
	c.Resume()
	c.Resume()
	require.False(t, c.IsPaused())
}

// func Test_PauseResume_Multithreaded(t *testing.T) {
// TODO(thampiotr): implement this test
// 	ctx := componenttest.TestContext(t)
// 	routines := 10
//
// 	pauses
//
// 	for i := 0; i < routines; i++ {
// 		go func() {
// 			for {
// 				select {
// 				case <-ctx.Done():
// 					return
// 				}
// 			}
// 		}()
// 	}
//
//
//
// }
