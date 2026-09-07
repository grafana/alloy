package stages

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripedMap(t *testing.T) {
	t.Run("Update sees zero value for a fresh key", func(t *testing.T) {
		m := newStripedMap[int](0)

		var seen int
		m.Update(1, func(v int) int {
			seen = v
			return v + 1
		})

		require.Zero(t, seen)

		got, ok := m.Take(1)
		require.True(t, ok)
		require.Equal(t, 1, got)
	})

	t.Run("Update sees the previous value", func(t *testing.T) {
		m := newStripedMap[int](0)

		m.Update(1, func(v int) int { return v + 1 })
		m.Update(1, func(v int) int { return v + 1 })
		m.Update(1, func(v int) int { return v + 1 })

		got, ok := m.Take(1)
		require.True(t, ok)
		require.Equal(t, 3, got)
	})

	t.Run("Take on a missing key returns false", func(t *testing.T) {
		m := newStripedMap[int](0)

		v, ok := m.Take(1)
		require.False(t, ok)
		require.Zero(t, v)
	})

	t.Run("Take removes the key", func(t *testing.T) {
		m := newStripedMap[string](0)
		m.Update(1, func(string) string { return "a" })

		v, ok := m.Take(1)
		require.True(t, ok)
		require.Equal(t, "a", v)

		v, ok = m.Take(1)
		require.False(t, ok)
		require.Empty(t, v)
	})

	t.Run("DrainIfAtLeast below threshold does nothing", func(t *testing.T) {
		m := newStripedMap[int](0)
		m.Update(1, func(int) int { return 1 })
		m.Update(2, func(int) int { return 2 })

		require.Nil(t, m.DrainIfAtLeast(3))

		got, ok := m.Take(1)
		require.True(t, ok)
		require.Equal(t, 1, got)
		got, ok = m.Take(2)
		require.True(t, ok)
		require.Equal(t, 2, got)
	})

	t.Run("DrainIfAtLeast at threshold drains everything", func(t *testing.T) {
		m := newStripedMap[int](0)
		m.Update(1, func(int) int { return 1 })
		m.Update(2, func(int) int { return 2 })

		drained := m.DrainIfAtLeast(2)
		require.Equal(t, []int{1, 2}, drained)
		require.Empty(t, m.DrainAll())
	})

	t.Run("DrainAll drains everything and empties the map", func(t *testing.T) {
		m := newStripedMap[int](0)
		for k := uint64(0); k < 20; k++ {
			k := k
			m.Update(k, func(int) int { return int(k) })
		}

		drained := m.DrainAll()
		require.Len(t, drained, 20)
		require.Empty(t, m.DrainAll())
	})

	t.Run("concurrent Update on the same key merges every call", func(t *testing.T) {
		const numGoroutines = 10
		m := newStripedMap[int](0)

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Go(func() {
				m.Update(42, func(v int) int { return v + 1 })
			})
		}
		wg.Wait()

		got, ok := m.Take(42)
		require.True(t, ok)
		require.Equal(t, numGoroutines, got)
	})

	t.Run("concurrent DrainIfAtLeast never loses or duplicates entries", func(t *testing.T) {
		const (
			numKeys   = 500
			threshold = 10
		)
		m := newStripedMap[int](numKeys)

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			drained []int
		)

		for k := uint64(0); k < numKeys; k++ {
			wg.Go(func() {
				m.Update(k, func(int) int { return int(k) })
				if got := m.DrainIfAtLeast(threshold); len(got) > 0 {
					mu.Lock()
					drained = append(drained, got...)
					mu.Unlock()
				}
			})
		}
		wg.Wait()

		drained = append(drained, m.DrainAll()...)

		want := make([]int, numKeys)
		for i := range want {
			want[i] = i
		}
		require.ElementsMatch(t, want, drained)
	})
}
