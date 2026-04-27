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
//
// The returned consumer owns background worker goroutines for its lifetime and
// must be stopped when no longer needed.
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

// ShardingConsumer shareds each stream to a shard based on the stream's labels.
//
// ShardingConsumer is intended to be scoped to the lifetime of a component.
// Callers must stop all log-producing goroutines before calling Stop. After
// Stop begins, Consume and ConsumeEntry reject late calls with
// ErrConsumerStopped.
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

	return s.joinErrors(ctx, errChans)
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

	return s.joinErrors(ctx, []<-chan error{errCh})
}

// Stop stops the shard workers and waits for them to exit.
//
// Callers must ensure all producers have stopped before calling Stop. Any late
// calls to Consume or ConsumeEntry after shutdown begins return
// ErrConsumerStopped.
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

func (s *ShardingConsumer) joinErrors(ctx context.Context, errChans []<-chan error) error {
	var errs []error

	for _, errCh := range errChans {
		select {
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		case err := <-errCh:
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
