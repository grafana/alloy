// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xsync_test

import (
	"bytes"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
)

type SharedResourceMutable struct {
	somethingThatNeedsLocking uint64
}

type SharedResource struct {
	mutable                                    xsync.RWMutex[SharedResourceMutable]
	atomicStateThatDoesntNeedAdditionalLocking atomic.Uint64
}

func TestRWMutex(t *testing.T) {
	// Data is split into two halves: the portion that needs locking and the portion that is either
	// constant or intrinsically synchronized (e.g. atomics).
	sharedResource := SharedResource{
		mutable: xsync.NewRWMutex(SharedResourceMutable{
			somethingThatNeedsLocking: 891723,
		}),
		atomicStateThatDoesntNeedAdditionalLocking: atomic.Uint64{},
	}

	mutable := sharedResource.mutable.RLock()
	mutable.somethingThatNeedsLocking += 123
	sharedResource.mutable.RUnlock(&mutable)
	// RUnlock zeros the reference to make sure we can't accidentally use it after unlocking.
	assert.Nil(t, mutable)
}

func TestRWMutex_ReferenceType(t *testing.T) {
	buf := bytes.NewBufferString("hello")

	b := xsync.NewRWMutex(buf.Bytes())
	mutable := b.WLock()
	*mutable = append(*mutable, []byte("world")...)
	b.WUnlock(&mutable)

	afterMutation := b.RLock()
	defer b.RUnlock(&afterMutation)
	assert.Equal(t, *afterMutation, []byte("helloworld"))
}

func ExampleRWMutex_WLock() {
	m := xsync.NewRWMutex(uint64(0))
	p := m.WLock()
	*p = 123
	// Copy the reference, defeating the pointer invalidation in `WUnlock. Do NOT do this.
	p2 := p
	m.WUnlock(&p)

	// We can incorrectly still write the data without holding the actual lock:
	*p2 = 345
}

func TestRWMutex_CrashOnUseAfterUnlock(t *testing.T) {
	m := xsync.NewRWMutex(uint64(0))
	p := m.WLock()
	*p = 123
	m.WUnlock(&p)

	assert.Panics(t, func() {
		*p = 345
	})
}
