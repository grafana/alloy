package loki

import (
	"context"
	"errors"
	"sync"

	"github.com/prometheus/common/model"
)

var _ Consumer = (*ShardingConsumer)(nil)

var ErrConsumerStopped = errors.New("consumer stopped")

// NewShardingConsumer creates a Consumer which shards streams across a fixed
// number of shards before forwarding them to the downstream consumer.
func NewShardingConsumer(shards int, consumer Consumer) *ShardingConsumer {
	if shards <= 0 {
		shards = 1
	}

	s := &ShardingConsumer{
		consumer: consumer,
		shards:   make([]chan shardingRequest, shards),
		done:     make(chan struct{}),
	}

	for i := range s.shards {
		s.shards[i] = make(chan shardingRequest)
		s.wg.Go(func() { s.runShard(s.shards[i]) })
	}

	return s
}

// ShardingConsumer serializes work per shard while allowing different shards to
// make progress independently. It is intended to be scoped to the lifetime of a
// component, and callers must stop all log-producing goroutines before calling
// Stop. Once a batch or entry has been accepted by a shard, execution is handed
// off to the downstream consumer.
type ShardingConsumer struct {
	shards []chan shardingRequest

	consumer Consumer

	stopOnce sync.Once
	wg       sync.WaitGroup
	done     chan struct{}
}

// Consume shards each stream in the batch to a shard and waits for all to finish.
// It returns ErrConsumerStopped if called after shutdown begins.
func (s *ShardingConsumer) Consume(ctx context.Context, batch Batch) error {
	streamLen := batch.StreamLen()
	if streamLen == 0 {
		return nil
	}

	errChans := make([]<-chan error, 0, streamLen)

	// NOTE: when we only have one stream in a batch there is no need to split it.
	if streamLen == 1 {
		errChans = append(errChans, s.consume(ctx, s.shardFor(batch.streams[0].Labels), batch))
	} else {
		batch.ConsumeStreams(func(stream Stream, created int64) {
			streamBatch := NewBatchWithCreatedUnixMicro(created)
			streamBatch.Add(stream)
			errChans = append(errChans, s.consume(ctx, s.shardFor(stream.Labels), streamBatch))
		})
	}

	return s.joinErrors(errChans)
}

func (s *ShardingConsumer) consume(ctx context.Context, shard int, batch Batch) <-chan error {
	errCh := make(chan error, 1)
	req := shardingRequest{
		errCh: errCh,
		consume: func(consumer Consumer) error {
			return consumer.Consume(ctx, batch)
		},
	}

	select {
	case <-ctx.Done():
		errCh <- ctx.Err()
	case <-s.done:
		errCh <- ErrConsumerStopped
	case s.shards[shard] <- req:
	}

	return errCh
}

// ConsumeEntry shards a single entry to the shard selected by the entry's labels.
// It returns ErrConsumerStopped if called after shutdown begins.
func (s *ShardingConsumer) ConsumeEntry(ctx context.Context, entry Entry) error {
	errCh := make(chan error, 1)
	req := shardingRequest{
		errCh: errCh,
		consume: func(consumer Consumer) error {
			return consumer.ConsumeEntry(ctx, entry)
		},
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return ErrConsumerStopped
	case s.shards[s.shardFor(entry.Labels)] <- req:
	}

	return s.joinErrors([]<-chan error{errCh})
}

// Stop stops the shards and waits for them to exit.
func (s *ShardingConsumer) Stop() {
	s.stopOnce.Do(func() {
		close(s.done)
		s.wg.Wait()
	})
}

func (s *ShardingConsumer) shardFor(labels model.LabelSet) int {
	return int(labels.FastFingerprint() % model.Fingerprint(len(s.shards)))
}

func (s *ShardingConsumer) runShard(shard <-chan shardingRequest) {
	for {
		select {
		case <-s.done:
			return
		case req := <-shard:
			req.errCh <- req.consume(s.consumer)
		}
	}
}

type shardingRequest struct {
	consume func(Consumer) error
	errCh   chan<- error
}

func (s *ShardingConsumer) joinErrors(errChans []<-chan error) error {
	errs := make([]error, 0, len(errChans))

	for _, errCh := range errChans {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
