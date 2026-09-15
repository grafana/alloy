package source

import (
	"slices"
	"testing"

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
		)
		require.Equal(t, 2, s.Len())
	})

	t.Run("should prevent duplicated source from being scheduled", func(t *testing.T) {
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
		)
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
		)
		require.Equal(t, 2, s.Len())
	})
}

func TestReconcileWithDedup(t *testing.T) {
	type testInput struct {
		key   int
		dedup string
	}

	s := NewScheduler[int]()
	defer s.Stop()

	t.Run("should reconcile all new sources", func(t *testing.T) {
		ReconcileWithDedup(
			logging.NewSlogNop(),
			s,
			slices.Values([]testInput{{key: 1, dedup: "a"}, {key: 2, dedup: "b"}, {key: 3, dedup: "c"}}),
			func(in testInput) int {
				return in.key
			},
			func(in testInput) string {
				return in.dedup
			},
			func(key int, in testInput) (Source[int], error) {
				return newTestSource(key, false), nil
			},
		)
		require.Equal(t, 3, s.Len())
	})

	t.Run("should stop all missing sources", func(t *testing.T) {
		ReconcileWithDedup(
			logging.NewSlogNop(),
			s,
			slices.Values([]testInput{{key: 2, dedup: "b"}, {key: 3, dedup: "c"}}),
			func(in testInput) int {
				return in.key
			},
			func(in testInput) string {
				return in.dedup
			},
			func(key int, in testInput) (Source[int], error) {
				return newTestSource(key, false), nil
			},
		)
		require.Equal(t, 2, s.Len())
		require.False(t, s.Contains(1))
	})

	t.Run("should only schedule first input that shares a dedup key", func(t *testing.T) {
		ReconcileWithDedup(
			logging.NewSlogNop(),
			s,
			slices.Values([]testInput{{key: 2, dedup: "b"}, {key: 3, dedup: "c"}, {key: 4, dedup: "c"}}),
			func(in testInput) int {
				return in.key
			},
			func(in testInput) string {
				return in.dedup
			},
			func(key int, in testInput) (Source[int], error) {
				return newTestSource(key, false), nil
			},
		)
		require.Equal(t, 2, s.Len())
		require.True(t, s.Contains(3))
		require.False(t, s.Contains(4))
	})
}
