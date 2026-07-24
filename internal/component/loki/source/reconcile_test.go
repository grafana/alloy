package source

import (
	"slices"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/stretchr/testify/require"
)

func TestReconcile(t *testing.T) {
	s := NewScheduler[int]()
	defer s.Stop()

	t.Run("should reconcile all new sources", func(t *testing.T) {
		Reconcile(
			logging.NewSlogNop(),
			s,
			slices.Values([]int{1, 2, 3}),
			func(v int) int {
				return v
			},
			func(key int, target int) (Source[int], error) {
				return newTestSource(key, false), nil
			},
			nil,
		)
		require.Equal(t, 3, s.Len())
	})

	t.Run("should stop all missing sources", func(t *testing.T) {
		Reconcile(
			logging.NewSlogNop(),
			s,
			slices.Values([]int{2, 3}),
			func(v int) int {
				return v
			},
			func(key int, target int) (Source[int], error) {
				return newTestSource(key, false), nil
			},
			nil,
		)
		require.Equal(t, 2, s.Len())
	})

	t.Run("should process a duplicated key once", func(t *testing.T) {
		var updated []int
		Reconcile(
			logging.NewSlogNop(),
			s,
			slices.Values([]int{2, 2, 3}),
			func(v int) int {
				return v
			},
			func(key int, target int) (Source[int], error) {
				return newTestSource(key, false), nil
			},
			func(existing Source[int], target int) (Source[int], error) {
				updated = append(updated, target)
				return nil, nil
			},
		)
		require.Equal(t, []int{2, 3}, updated)
		require.Equal(t, 2, s.Len())
	})

	t.Run("should not schedule if error is returned", func(t *testing.T) {
		Reconcile(
			logging.NewSlogNop(),
			s,
			slices.Values([]int{2, 3, 4}),
			func(v int) int {
				return v
			},
			func(key int, target int) (Source[int], error) {
				if key == 4 {
					return nil, ErrSkip
				}
				return newTestSource(key, false), nil
			},
			nil,
		)
		require.Equal(t, 2, s.Len())
	})

	t.Run("should update an existing source", func(t *testing.T) {
		var updated []int
		Reconcile(
			logging.NewSlogNop(),
			s,
			slices.Values([]int{2, 3}),
			func(v int) int {
				return v
			},
			func(key int, target int) (Source[int], error) {
				return newTestSource(key, false), nil
			},
			func(existing Source[int], target int) (Source[int], error) {
				updated = append(updated, target)
				return nil, nil
			},
		)
		require.ElementsMatch(t, []int{2, 3}, updated)
		require.Equal(t, 2, s.Len())
	})

	t.Run("should replace an existing source", func(t *testing.T) {
		before, ok := s.GetSource(2)
		require.True(t, ok)
		require.Eventually(t, before.(*testSource).IsRunning, time.Second, 10*time.Millisecond)

		Reconcile(
			logging.NewSlogNop(),
			s,
			slices.Values([]int{2, 3}),
			func(v int) int {
				return v
			},
			func(key int, target int) (Source[int], error) {
				return newTestSource(key, false), nil
			},
			func(existing Source[int], target int) (Source[int], error) {
				if target == 2 {
					return newTestSource(target, false), nil
				}
				return nil, nil
			},
		)

		after, ok := s.GetSource(2)
		require.True(t, ok)
		require.NotSame(t, before, after)
		require.False(t, before.(*testSource).IsRunning(),
			"the old source must stop before its replacement is scheduled")
		require.Equal(t, 2, s.Len())
	})
}
