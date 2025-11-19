// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xsync // import "go.opentelemetry.io/ebpf-profiler/libpf/xsync"

import "sync"

// RWMutex is a thin wrapper around sync.RWMutex that hides away the data it protects to ensure it's
// not accidentally accessed without actually holding the lock.
//
// The design is inspired by how Rust implement its locks.
//
// Given Go's weak type system it's not able to provide perfect safety, but it at least clearly
// communicates to developers exactly which resources are protected by which lock without having to
// sift through documentation (or code, if documentation doesn't exist).
//
// To better demonstrate how this abstraction helps to avoid mistakes, consider the following
// example struct implementing an object manager of some sort:
//
//	type ID uint64
//
//	type SomeObject struct {
//		// ...
//	}
//
//	type ObjectManager struct {
//		objects map[ID]*SomeObject
//	}
//
//	func (mgr *ObjectManager) AddObject(id ID, obj *SomeObject) {
//		mgr.objects[id] = obj
//	}
//
//	func (mgr *ObjectManager) RemoveObject(id ID) {
//		delete(mgr.objects, id)
//	}
//
//	func (mgr *ObjectManager) GetObject(id ID) *SomeObject {
//		x := mgr.objects[id]
//		return x
//	}
//
// Now you want to rework the public interface of ObjectManager to be thread-safe. The perhaps most
// obvious solution would be to just add `mu sync.RWMutex` to ObjectManager and lock it immediately
// when entering each public function:
//
//	type ID uint64
//
//	type SomeObject struct {
//		// ...
//	}
//
//	type ObjectManager struct {
//		mu      sync.RWMutex
//		objects map[ID]*SomeObject
//	}
//
//	func (mgr *ObjectManager) AddObject(id ID, obj *SomeObject) {
//		mgr.mu.Lock()
//		mgr.mu.Unlock() // <- oh no, forgot to write `defer`!
//		mgr.objects[id] = obj
//	}
//
//	func (mgr *ObjectManager) RemoveObject(id ID) {
//		// oh no, forgot to take the lock entirely!
//		delete(mgr.objects, id)
//	}
//
//	func (mgr *ObjectManager) GetObject(id ID) *SomeObject {
//		mgr.mu.RLock()
//		defer mgr.mu.RUnlock()
//		return mgr.objects[id]
//	}
//
// Unfortunately, we made two mistakes in our implementation. The code will however likely still
// pass all kinds of tests, simply because it's very hard to write tests that detect race
// conditions in tests.
//
// Now, the same thing using xsync.RWMutex instead:
//
//	type ID uint64
//
//	type SomeObject struct {
//		// ...
//	}
//
//	type ObjectManager struct {
//		objects xsync.RWMutex[map[ID]*SomeObject]
//	}
//
//	func (mgr *ObjectManager) AddObject(id ID, obj *SomeObject) {
//		var *SomeObject objects := mgr.objects.RLock()
//		mgr.objects.RUnlock(&objects) // <- oh no, forgot to write `defer`!
//		objects[id] = obj             // <- will immediately crash in tests
//		                              //    because `RUnlock` set our pointer to `nil`
//	}
//
//	func (mgr *ObjectManager) RemoveObject(id ID) {
//		// oh no, forgot to take the lock entirely! With xsync.RWMutex, this won't
//		// compile: there simply is no direct pointer to the protected data that we
//		// could use to accidentally access shared data without going through calling
//		// `RLock`/`WLock` first.
//		delete(mgr.objects, id)
//	}
//
//	func (mgr *ObjectManager) GetObject(id ID) *SomeObject {
//		objects := mgr.mu.RLock()
//		defer mgr.mu.RUnlock(&objects)
//		return mgr.objects[id]
//	}
type RWMutex[T any] struct {
	guarded T
	mutex   sync.RWMutex
}

// NewRWMutex creates a new read-write mutex.
func NewRWMutex[T any](guarded T) RWMutex[T] {
	return RWMutex[T]{
		guarded: guarded,
	}
}

// RLock locks the mutex for reading, returning a pointer to the protected data.
//
// The caller **must not** write to the data pointed to by the returned pointer.
//
// Further, the caller **must not** let the returned pointer leak out of the scope of the function
// where it was originally created, except for temporarily borrowing it to other functions. The
// caller must make sure that callees never save this pointer anywhere.
func (mtx *RWMutex[T]) RLock() *T {
	mtx.mutex.RLock()
	return &mtx.guarded
}

// RUnlock unlocks the mutex after previously being locked by RLock.
//
// Pass a reference to the pointer returned from RLock here to ensure it is invalidated.
func (mtx *RWMutex[T]) RUnlock(ref **T) {
	*ref = nil
	mtx.mutex.RUnlock()
}

// WLock locks the mutex for writing, returning a pointer to the protected data.
//
// The caller **must not** let the returned pointer leak out of the scope of the function where it
// was originally created, except for temporarily borrowing it to other functions. The caller must
// make sure that callees never save this pointer anywhere.
func (mtx *RWMutex[T]) WLock() *T {
	mtx.mutex.Lock()
	return &mtx.guarded
}

// WUnlock unlocks the mutex after previously being locked by WLock.
//
// Pass a reference to the pointer returned from WLock here to ensure it is invalidated.
func (mtx *RWMutex[T]) WUnlock(ref **T) {
	*ref = nil
	mtx.mutex.Unlock()
}
