package runner

import "sync"

type hashMap struct {
	mut    sync.RWMutex
	lookup map[uint64][]Task
}

// newHashMap creates a new hashMap allocated to handle at least size unique
// hashes.
func newHashMap(size int) *hashMap {
	return &hashMap{
		lookup: make(map[uint64][]Task, size),
	}
}

// Add adds the provided Task into the hashMap. It returns false if t
// already exists in the hashMap.
func (hm *hashMap) Add(t Task) bool {
	hm.mut.Lock()
	defer hm.mut.Unlock()

	hash := t.Hash()
	for _, compare := range hm.lookup[hash] {
		if compare.Equals(t) {
			return false
		}
	}

	hm.lookup[hash] = append(hm.lookup[hash], t)
	return true
}

// Has returns true if t exists in the hashMap.
func (hm *hashMap) Has(t Task) bool {
	hm.mut.RLock()
	defer hm.mut.RUnlock()

	for _, compare := range hm.lookup[t.Hash()] {
		if compare.Equals(t) {
			return true
		}
	}

	return false
}

// Delete removes the provided Task from the hashMap. It returns true if the
// Task was found and deleted.
func (hm *hashMap) Delete(t Task) (deleted bool) {
	hm.mut.Lock()
	defer hm.mut.Unlock()

	hash := t.Hash()

	var remaining []Task
	for _, s := range hm.lookup[hash] {
		if s.Equals(t) {
			deleted = true
			continue
		}
		remaining = append(remaining, s)
	}
	if len(remaining) == 0 {
		delete(hm.lookup, hash)
	} else {
		hm.lookup[hash] = remaining
	}

	return deleted
}

// Iterate returns a channel which iterates through all elements in the
// hashMap. The channel *must* be fully consumed, otherwise the hashMap will
// deadlock.
//
// The iteration order is not guaranteed.
func (hm *hashMap) Iterate() <-chan Task {
	taskCh := make(chan Task)

	go func() {
		hm.mut.Lock()
		defer hm.mut.Unlock()

		for _, set := range hm.lookup {
			for _, task := range set {
				taskCh <- task
			}
		}

		close(taskCh)
	}()

	return taskCh
}
