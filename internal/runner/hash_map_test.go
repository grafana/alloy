package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_hashMap(t *testing.T) {
	t.Run("tasks can be added", func(t *testing.T) {
		hm := newHashMap(0)

		// Try adding the task twice. The first time we add it, Add should return
		// true, since it's brand new to the hash map. The second time we add it,
		// Add should return false, since the task was previously added.
		task := newBasicMockTask(1)
		require.True(t, hm.Add(task), "Add should have returned true on the first call")
		require.False(t, hm.Add(task), "Add should have returned false on the second call of the same task")

		// Try adding a new task twice.
		otherTask := newBasicMockTask(2)
		require.True(t, hm.Add(otherTask), "Add should have returned true on the first call")
		require.False(t, hm.Add(otherTask), "Add should have returned false on the second call of the same task")

		// Make sure that both of our added tasks can be returned.
		tasks := collectMockTasks(hm)
		require.ElementsMatch(t, tasks, []*mockTask{task, otherTask})
	})

	t.Run("tasks can be searched", func(t *testing.T) {
		hm := newHashMap(0)

		var (
			task1 = newBasicMockTask(1)
			task2 = newBasicMockTask(2)
		)

		// Add task1 into the hashMap, but not task2.
		require.True(t, hm.Add(task1))

		require.True(t, hm.Has(task1), "task1 should be in the hashMap")
		require.False(t, hm.Has(task2), "task2 should not be in the hashMap")
	})

	t.Run("tasks can be deleted", func(t *testing.T) {
		hm := newHashMap(0)

		// Create two sets of tasks, where each set has exactly one hash collision
		// with the other set. This helps us test that deletes work when there's
		// collisions too.
		var tasks []*mockTask
		for i := 0; i < 10; i++ {
			tasks = append(tasks, newBasicMockTask(uint64(i)))
		}
		for i := 0; i < 10; i++ {
			tasks = append(tasks, newBasicMockTask(uint64(i)))
		}

		// Add tasks all at once.
		for _, task := range tasks {
			require.True(t, hm.Add(task))
		}

		// Start removing every task until there's none left.
		expectCount := 20
		for _, task := range tasks {
			require.Len(t, collectMockTasks(hm), expectCount)
			expectCount--

			require.True(t, hm.Delete(task))
		}

		require.Len(t, collectMockTasks(hm), 0)
	})

	t.Run("tasks are deduplicated", func(t *testing.T) {
		hm := newHashMap(0)

		// Create two tasks with the same hash and that report they're equal to
		// each other, even though they're separate pointers.
		//
		// The hashMap should deduplicate these tasks and only store one.
		var (
			task1 = &mockTask{
				HashFunc:   func() uint64 { return 1 },
				EqualsFunc: func(t Task) bool { return true },
			}
			task2 = &mockTask{
				HashFunc:   func() uint64 { return 1 },
				EqualsFunc: func(t Task) bool { return true },
			}
		)

		require.True(t, hm.Add(task1))
		require.False(t, hm.Add(task2))

		// The hashMap should say that both task1 and task2 exist because they are
		// considered duplicates.
		require.True(t, hm.Has(task1))
		require.True(t, hm.Has(task2))

		// Make sure that both of our added tasks can be returned.
		tasks := collectMockTasks(hm)
		require.ElementsMatch(t, tasks, []*mockTask{task1})
	})

	t.Run("hash collisions are handled", func(t *testing.T) {
		hm := newHashMap(0)

		// Create two tasks with the same hash but that aren't equal to each other
		// (because mockTask.Equals will check for pointer equality).
		//
		// It should be possible to store both tasks in the hashMap, despite having
		// the same hash.
		var (
			task1 = newBasicMockTask(1)
			task2 = newBasicMockTask(1)
		)

		require.True(t, hm.Add(task1))
		require.True(t, hm.Add(task2))

		require.True(t, hm.Has(task1))
		require.True(t, hm.Has(task2))

		// Make sure that both of our added tasks can be returned.
		tasks := collectMockTasks(hm)
		require.ElementsMatch(t, tasks, []*mockTask{task1, task2})
	})
}

func Benchmark_hashMap(b *testing.B) {
	b.Run("Add element", func(b *testing.B) {
		task := newBasicMockTask(0)

		for i := 0; i < b.N; i++ {
			hm := newHashMap(1)
			hm.Add(task)
		}
	})

	b.Run("Remove element", func(b *testing.B) {
		task := newBasicMockTask(0)

		// Precreate the hash maps to only measure deletes.
		var hashMaps []*hashMap
		for i := 0; i < b.N; i++ {
			hm := newHashMap(1)
			hm.Add(task)
			hashMaps = append(hashMaps, hm)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hashMaps[i].Delete(task)
		}
	})

	b.Run("Lookup element", func(b *testing.B) {
		task := newBasicMockTask(0)

		// Pre-add the task into the hash map so it's not counted as part of the
		// performance benchmark.
		hm := newHashMap(0)
		hm.Add(task)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			hm.Has(task)
		}
	})

	b.Run("Add existing element", func(b *testing.B) {
		task := newBasicMockTask(0)

		// Pre-add the task into the hash map so it's not counted as part of the
		// performance benchmark.
		hm := newHashMap(0)
		hm.Add(task)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			hm.Add(task)
		}
	})
}

func Benchmark_hashMap_collisions(b *testing.B) {
	collisionChecks := []int{1, 2, 5, 10, 100}
	for _, collisions := range collisionChecks {
		b.Run(fmt.Sprintf("%d collisions", collisions), func(b *testing.B) {
			// Precreate the set of tasks. Each task should have the hash 0 to check
			// for the hash collision handling performance.
			var tasks []*mockTask
			for i := 0; i < collisions+1; i++ {
				tasks = append(tasks, newBasicMockTask(0))
			}

			b.Run("Add", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					hm := newHashMap(0)
					for _, task := range tasks {
						hm.Add(task)
					}
				}
			})

			b.Run("Remove", func(b *testing.B) {
				hm := newHashMap(0)
				for _, task := range tasks {
					hm.Add(task)
				}
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					// Iterate in reverse to get worse-case performance.
					for i := len(tasks) - 1; i >= 0; i-- {
						hm.Delete(tasks[i])
					}
				}
			})

			b.Run("Lookup", func(b *testing.B) {
				hm := newHashMap(0)
				for _, task := range tasks {
					hm.Add(task)
				}
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					for _, task := range tasks {
						hm.Has(task)
					}
				}
			})
		})
	}
}

func collectMockTasks(hm *hashMap) []*mockTask {
	var res []*mockTask
	for t := range hm.Iterate() {
		res = append(res, t.(*mockTask))
	}
	return res
}

type mockTask struct {
	HashFunc   func() uint64
	EqualsFunc func(t Task) bool
}

func newBasicMockTask(hash uint64) *mockTask {
	return &mockTask{
		HashFunc: func() uint64 { return hash },
	}
}

var _ Task = (*mockTask)(nil)

func (mt *mockTask) Hash() uint64 {
	if mt.HashFunc == nil {
		return 0
	}
	return mt.HashFunc()
}

func (mt *mockTask) Equals(t Task) bool {
	if mt.EqualsFunc == nil {
		// Default to pointer comparison.
		return mt == t.(*mockTask)
	}
	return mt.EqualsFunc(t)
}
