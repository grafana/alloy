package batch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/prometheus/prompb"
)

type writer struct {
	mut      sync.RWMutex
	parentId string
	to       *QueueManager
	store    *filequeue
	ctx      context.Context
	l        log.Logger
}

func newWriter(parent string, to *QueueManager, store *filequeue, l log.Logger) *writer {
	name := fmt.Sprintf("metrics_write_to_%s_parent_%s", to.storeClient.Name(), parent)
	w := &writer{
		parentId: parent,
		to:       to,
		store:    store,
		l:        log.With(l, "name", name),
	}
	return w
}

func (w *writer) Start(ctx context.Context) {
	w.mut.Lock()
	w.ctx = ctx
	w.mut.Unlock()

	success := false
	more := false
	found := false

	var valByte []byte
	var handle string

	for {
		timeOut := 1 * time.Second
		valByte, handle, found, more = w.store.Next(valByte[:0])
		if found {
			var recoverableError bool
			success, recoverableError = w.send(valByte, ctx)
			// We need to succeed or hit an unrecoverable error to move on.
			if success || !recoverableError {
				w.store.Delete(handle)
			}
		}

		// If we were successful and nothing is in the queue
		// If the queue is not full then give time for it to send.
		if success && more {
			timeOut = 10 * time.Millisecond
		}

		tmr := time.NewTimer(timeOut)
		select {
		case <-w.ctx.Done():
			return
		case <-tmr.C:
			continue
		}
	}
}

var wrPool = sync.Pool{New: func() any {
	return &prompb.WriteRequest{}
}}

func (w *writer) send(val []byte, ctx context.Context) (success bool, recoverableError bool) {
	recoverableError = true

	var err error
	wr := wrPool.Get().(*prompb.WriteRequest)
	defer wrPool.Put(wr)

	// TODO add setting to handle wal age.
	dur, _ := time.ParseDuration("36h")
	d, err := DeserializeParquet(val, int64(dur.Seconds()))
	if err != nil {
		return false, false
	}
	success = w.to.Append(d)
	if err != nil {
		// Let's check if it's an `out of order sample`. Yes this is some hand waving going on here.
		// TODO add metric for unrecoverable error
		if strings.Contains(err.Error(), "the sample has been rejected") {
			recoverableError = false
		}
		level.Error(w.l).Log("msg", "error sending samples", "err", err)
	}
	return success, recoverableError
}
