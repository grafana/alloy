package queue

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/prometheus/prompb"
)

type remoteWriter struct {
	mut          sync.RWMutex
	parentId     string
	to           *QueueManager
	store        *filequeue
	ctx          context.Context
	l            log.Logger
	ttl          time.Duration
	writeByte    prometheus.Gauge
	writeMetrics prometheus.Gauge
}

func newRemoteWriter(parent string, to *QueueManager, store *filequeue, l log.Logger, ttl time.Duration, register prometheus.Registerer) *remoteWriter {
	name := fmt.Sprintf("metrics_write_to_%s_parent_%s", to.storeClient.Name(), parent)
	w := &remoteWriter{
		parentId: parent,
		to:       to,
		store:    store,
		l:        log.With(l, "name", name),
		ttl:      ttl,
		writeByte: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alloy_remote_write_queue_send_bytes",
			Help: "The number of bytes sent to the remote write.",
			ConstLabels: map[string]string{
				"remote": to.storeClient.Name(),
			},
		}),
		writeMetrics: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alloy_remote_write_queue_send_samples",
			Help: "The number of samples sent to the remote write.",
			ConstLabels: map[string]string{
				"remote": to.storeClient.Name(),
			},
		}),
	}
	register.Register(w.writeByte)
	register.Register(w.writeMetrics)

	return w
}

func (w *remoteWriter) Start(ctx context.Context) {
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
			success = w.send(valByte, ctx)
			// We need to succeed or hit an unrecoverable error to move on.
			if success {
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

func (w *remoteWriter) send(val []byte, ctx context.Context) bool {
	var err error
	wr := wrPool.Get().(*prompb.WriteRequest)
	defer wrPool.Put(wr)

	d, err := Deserialize(val, int64(w.ttl.Seconds()))
	if err != nil {
		return false
	}
	w.writeByte.Add(float64(len(val)))
	w.writeMetrics.Add(float64(len(d)))
	success := w.to.Append(d)
	if err != nil {
		level.Error(w.l).Log("msg", "error sending samples", "err", err)
	}
	return success
}
