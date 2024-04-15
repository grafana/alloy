package batch

import (
	"bytes"
	"context"
	"fmt"
	"math"
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
	l := LinearPool.Get().(*batch)
	defer l.Reset()
	defer LinearPool.Put(l)
	wr := wrPool.Get().(*prompb.WriteRequest)
	defer wrPool.Put(wr)

	// TODO add setting to handle wal age.
	d, err := l.Deserialize(bytes.NewBuffer(val), math.MaxInt64)
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

func makeWriteRequestDeserialized(samples []*deserializedMetric, wr *prompb.WriteRequest) {
	if len(samples) > len(wr.Timeseries) {
		wr.Timeseries = make([]prompb.TimeSeries, len(samples))
		for i := 0; i < len(samples); i++ {
			wr.Timeseries[i].Samples = make([]prompb.Sample, 1)
			wr.Timeseries[i].Samples[0] = prompb.Sample{}
			wr.Timeseries[i].Labels = make([]prompb.Label, 0)
		}
	}

	wr.Timeseries = wr.Timeseries[:len(samples)]
	for i, s := range samples {
		if cap(wr.Timeseries[i].Labels) < len(s.lbls) {
			wr.Timeseries[i].Labels = make([]prompb.Label, len(s.lbls))
		} else {
			wr.Timeseries[i].Labels = wr.Timeseries[i].Labels[:len(s.lbls)]
		}
		for x, l := range s.lbls {
			wr.Timeseries[i].Labels[x].Name = l.Name
			wr.Timeseries[i].Labels[x].Value = l.Value
		}
		wr.Timeseries[i].Samples[0].Value = s.val
		wr.Timeseries[i].Samples[0].Timestamp = s.ts
	}
}
