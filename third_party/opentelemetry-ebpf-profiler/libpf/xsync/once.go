// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xsync // import "go.opentelemetry.io/ebpf-profiler/libpf/xsync"

import (
	"sync"
	"sync/atomic"
)

// NOTE: synchronization logic closely borrowed from sync.Once

// Once is a lock that ensures that some data is initialized exactly once.
//
// Does not need explicit construction: simply do Once[MyType]{}.
type Once[T any] struct {
	done atomic.Bool
	mu   sync.Mutex
	data T
}

// GetOrInit the data protected by this lock.
//
// If the init function fails, the error is returned and the data is still
// considered to be uninitialized. The init function will then be called
// again on the next GetOrInit call. Only one thread will ever call init
// at the same time.
func (l *Once[T]) GetOrInit(init func() (T, error)) (*T, error) {
	if !l.done.Load() {
		// Outlined slow-path to allow inlining of the fast-path.
		return l.initSlow(init)
	}

	return &l.data, nil
}

func (l *Once[T]) initSlow(init func() (T, error)) (*T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Contending call might have initialized while we waited for the lock.
	if l.done.Load() {
		return &l.data, nil
	}

	var err error
	l.data, err = init()
	if err != nil {
		return nil, err
	}

	l.done.Store(true)
	return &l.data, err
}

// Get the previously initialized value.
//
// If the Once is not yet initialized, nil is returned.
func (l *Once[T]) Get() *T {
	if !l.done.Load() {
		return nil
	}

	return &l.data
}
