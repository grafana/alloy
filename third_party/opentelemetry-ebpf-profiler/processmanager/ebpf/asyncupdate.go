// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ebpf // import "go.opentelemetry.io/ebpf-profiler/processmanager/ebpf"

import (
	"context"
	"errors"
	"unsafe"

	cebpf "github.com/cilium/ebpf"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/host"
)

// asyncMapUpdaterPool is a pool of goroutines for doing non-blocking updates
// to BPF maps of the "map-in-map" type.
//
// This is necessary because BPF map-in-map updates have an unusually high
// latency compared to updates on other map types. They aren't computationally
// expensive, but they cause the kernel to call `synchronize_rcu` to ensure
// that the map update is actually in place before returning to user-land:
//
// https://elixir.bootlin.com/linux/v6.6.2/source/kernel/bpf/syscall.c#L142
//
// In the simplest terms `synchronize_rcu` can be thought of like a 15-30ms
// sleep that ensures that a change in memory has propagated into the caches
// of all CPU cores. This means that any map-in-map update through the bpf
// syscall will always take about equally long to return, causing significant
// slowdown during startup.
//
// The use case in our profiling agent really doesn't need these strict sync
// guarantees; we are perfectly happy with the update being performed in an
// eventually consistent fashion. We achieve this by spawning N background
// workers and routing update requests based on the key that is supposed to
// be updated.
//
// The partitioned queue design was chosen over a work-stealing queue to ensure
// that updates on individual keys are executed in sequential order. If we
// didn't do this, it could happen that a previously enqueued and delayed
// deletion is executed after an insertion (that we want to persist) or vice
// versa.
type asyncMapUpdaterPool struct {
	workers []*asyncUpdateWorker
}

// newAsyncMapUpdaterPool creates a new worker pool
func newAsyncMapUpdaterPool(ctx context.Context,
	numWorkers, workerQueueCapacity int) *asyncMapUpdaterPool {
	pool := &asyncMapUpdaterPool{}
	for range numWorkers {
		queue := make(chan asyncMapInMapUpdate, workerQueueCapacity)
		worker := &asyncUpdateWorker{ctx: ctx, queue: queue}
		go worker.serve()
		pool.workers = append(pool.workers, worker)
	}
	return pool
}

// EnqueueUpdate routes a map update request to a worker in the pool.
//
// Update requests for the same file ID are guaranteed to always be routed to
// the same worker. An `inner` value of `nil` requests deletion. Ownership of
// the given `inner` map is transferred to the worker pool and `inner` is closed
// by the background worker after the update was executed.
func (p *asyncMapUpdaterPool) EnqueueUpdate(
	outer *cebpf.Map, fileID host.FileID, inner *cebpf.Map) {
	workerIdx := uint64(fileID) % uint64(len(p.workers))
	if err := p.workers[workerIdx].ctx.Err(); err != nil {
		log.Warnf("Skipping handling of %v: %v", fileID, err)
		return
	}
	p.workers[workerIdx].queue <- asyncMapInMapUpdate{
		Outer:  outer,
		FileID: fileID,
		Inner:  inner,
	}
}

// asyncMapInMapUpdate is an asynchronous update request for a map-in-map BPF map.
type asyncMapInMapUpdate struct {
	Outer  *cebpf.Map
	FileID host.FileID
	Inner  *cebpf.Map // nil = delete
}

// asyncUpdateWorker represents a worker in a newAsyncMapUpdaterPool.
type asyncUpdateWorker struct {
	ctx   context.Context
	queue chan asyncMapInMapUpdate
}

// serve is the main loop of an update worker.
func (w *asyncUpdateWorker) serve() {
WorkerLoop:
	for {
		var update asyncMapInMapUpdate
		select {
		case <-w.ctx.Done():
			break WorkerLoop
		case update = <-w.queue:
		}

		var err error
		if update.Inner == nil {
			err = update.Outer.Delete(unsafe.Pointer(&update.FileID))
		} else {
			fd := uint32(update.Inner.FD())
			err = update.Outer.Update(unsafe.Pointer(&update.FileID),
				unsafe.Pointer(&fd), cebpf.UpdateNoExist)
			err = errors.Join(err, update.Inner.Close())
		}

		if err != nil {
			log.Warnf("Outer map update failure: %v", err)
		}
	}

	// Shutting down: drain remaining queue capacity & close the inner maps.
	for {
		select {
		case update := <-w.queue:
			_ = update.Inner.Close()
		default:
			return
		}
	}
}
