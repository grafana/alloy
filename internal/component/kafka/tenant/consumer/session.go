package consumer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// maxQueuedBatches is how many fetched batches a worker may have waiting
// before fetching its partition is paused.
const maxQueuedBatches = 2

type topicPartition struct {
	topic     string
	partition int32
}

func (tp topicPartition) asMap() map[string][]int32 {
	return map[string][]int32{tp.topic: {tp.partition}}
}

// session is one consumer group membership. Every assigned partition gets its
// own worker, so a partition whose downstream is stalled holds up only
// itself: the poll loop never waits for a worker, it pauses fetching the
// worker's partition instead.
type session struct {
	ctx context.Context
	c   *Component

	mut     sync.Mutex
	workers map[topicPartition]*worker
}

func newSession(ctx context.Context, c *Component) *session {
	return &session{ctx: ctx, c: c, workers: map[topicPartition]*worker{}}
}

// poll fetches records and hands them to the partitions' workers until the
// session's context is canceled.
func (s *session) poll(cl *kgo.Client) {
	for s.ctx.Err() == nil {
		fetches := cl.PollFetches(s.ctx)
		if fetches.IsClientClosed() {
			return
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			if !errors.Is(err, context.Canceled) {
				s.c.opts.Logger.Warn("fetch error", "topic", topic, "partition", partition, "err", err)
			}
		})

		s.mut.Lock()
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			if len(p.Records) == 0 {
				return
			}
			w, ok := s.workers[topicPartition{p.Topic, p.Partition}]
			if !ok {
				// Can't happen: rebalances are blocked until AllowRebalance.
				s.c.opts.Logger.Warn("fetched records for a partition without a worker", "topic", p.Topic, "partition", p.Partition)
				return
			}
			w.push(p.Records, p.HighWatermark)
		})
		s.mut.Unlock()
		cl.AllowRebalance()
	}
}

// shutdown stops all workers and commits what they finished.
func (s *session) shutdown(cl *kgo.Client) {
	s.mut.Lock()
	all := map[string][]int32{}
	for tp := range s.workers {
		all[tp.topic] = append(all[tp.topic], tp.partition)
	}
	s.mut.Unlock()
	s.stop(cl, all)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := cl.CommitMarkedOffsets(ctx); err != nil {
		s.c.opts.Logger.Warn("failed to commit offsets on shutdown; records will be reprocessed", "err", err)
	}
}

func (s *session) onAssigned(_ context.Context, cl *kgo.Client, assigned map[string][]int32) {
	s.c.metrics.rebalances.Inc()

	s.mut.Lock()
	defer s.mut.Unlock()
	for topic, parts := range assigned {
		for _, p := range parts {
			tp := topicPartition{topic, p}
			w := newWorker(s.ctx, s.c, cl, tp)
			s.workers[tp] = w
			go w.run()
		}
		// A partition may still be paused from an earlier assignment.
		cl.ResumeFetchPartitions(map[string][]int32{topic: parts})
		s.c.metrics.assignedPartitions.Add(float64(len(parts)))
		s.c.opts.Logger.Info("partitions assigned", "topic", topic, "partitions", fmt.Sprint(parts))
	}
}

func (s *session) onRevoked(ctx context.Context, cl *kgo.Client, revoked map[string][]int32) {
	s.stop(cl, revoked)
	// Commit what the stopped workers finished before the partitions move.
	if err := cl.CommitMarkedOffsets(ctx); err != nil {
		s.c.opts.Logger.Warn("failed to commit offsets on revoke; records will be reprocessed", "err", err)
	}
}

func (s *session) onLost(_ context.Context, cl *kgo.Client, lost map[string][]int32) {
	// Lost partitions may already belong to another member, so don't commit.
	s.stop(cl, lost)
}

// stop cancels the workers of parts and waits for them to exit. The rebalance
// doesn't complete until this returns, so a partition is never processed by
// two members at once.
func (s *session) stop(cl *kgo.Client, parts map[string][]int32) {
	var stopped []*worker

	s.mut.Lock()
	for topic, ps := range parts {
		for _, p := range ps {
			tp := topicPartition{topic, p}
			w, ok := s.workers[tp]
			if !ok {
				continue
			}
			delete(s.workers, tp)
			w.cancel()
			stopped = append(stopped, w)
			s.c.metrics.assignedPartitions.Dec()
			s.c.metrics.lag.DeleteLabelValues(topic, fmt.Sprint(p))
		}
		if len(ps) > 0 {
			// Don't leave a revoked partition paused for a later assignment.
			cl.ResumeFetchPartitions(map[string][]int32{topic: ps})
			s.c.opts.Logger.Info("partitions revoked", "topic", topic, "partitions", fmt.Sprint(ps))
		}
	}
	s.mut.Unlock()

	for _, w := range stopped {
		<-w.done
	}
}

type batch struct {
	records       []*kgo.Record
	highWatermark int64
}

// worker processes the records of one partition in order.
type worker struct {
	c  *Component
	cl *kgo.Client
	tp topicPartition

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{}

	mut    sync.Mutex
	queue  []batch
	paused bool
}

func newWorker(ctx context.Context, c *Component, cl *kgo.Client, tp topicPartition) *worker {
	ctx, cancel := context.WithCancel(ctx)
	return &worker{
		c:      c,
		cl:     cl,
		tp:     tp,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
		wake:   make(chan struct{}, 1),
	}
}

// push queues fetched records. It never blocks: once the worker falls
// behind, fetching its partition is paused instead.
func (w *worker) push(records []*kgo.Record, highWatermark int64) {
	w.mut.Lock()
	w.queue = append(w.queue, batch{records: records, highWatermark: highWatermark})
	if len(w.queue) >= maxQueuedBatches && !w.paused {
		w.cl.PauseFetchPartitions(w.tp.asMap())
		w.paused = true
	}
	w.mut.Unlock()

	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// next returns the next queued batch, resuming fetching once the queue is
// drained. It returns false once the worker is stopped.
func (w *worker) next() (batch, bool) {
	for {
		w.mut.Lock()
		if len(w.queue) > 0 {
			b := w.queue[0]
			w.queue = w.queue[1:]
			if len(w.queue) == 0 && w.paused {
				w.cl.ResumeFetchPartitions(w.tp.asMap())
				w.paused = false
			}
			w.mut.Unlock()
			return b, true
		}
		w.mut.Unlock()

		select {
		case <-w.ctx.Done():
			return batch{}, false
		case <-w.wake:
		}
	}
}

func (w *worker) run() {
	defer close(w.done)
	for {
		b, ok := w.next()
		if !ok || !w.process(b) {
			return
		}
	}
}

// process forwards a batch in order, marking each finished record for
// commit. It returns false if the worker was stopped mid-batch; the
// unfinished records are then reprocessed by the partition's next owner.
func (w *worker) process(b batch) bool {
	if hook := w.c.processHook; hook != nil {
		hook(w.tp.topic, w.tp.partition, true)
		defer hook(w.tp.topic, w.tp.partition, false)
	}

	args := w.c.currentArgs()
	for _, r := range b.records {
		if !w.c.processRecord(w.ctx, args, r) {
			return false
		}
		w.cl.MarkCommitRecords(r)
		w.c.metrics.lag.WithLabelValues(w.tp.topic, fmt.Sprint(w.tp.partition)).Set(float64(b.highWatermark - r.Offset - 1))
	}
	return true
}
