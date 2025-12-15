package source

import (
	"slices"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestReconcile(t *testing.T) {
	s := NewScheduler[int]()
	defer s.Stop()

	t.Run("should reconcile all new sources", func(t *testing.T) {
		Reconcile(
			log.NewNopLogger(),
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
			log.NewNopLogger(),
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
			log.NewNopLogger(),
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
			log.NewNopLogger(),
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
