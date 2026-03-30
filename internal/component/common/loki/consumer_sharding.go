package loki

import (
	"context"
	"errors"
	"sync"

	"github.com/prometheus/common/model"
)

var _ Consumer = (*ShardingConsumer)(nil)

// NewShardingConsumer creates a Consumer which shards streams across a fixed
// number of shards before forwarding them to the downstream consumer.
func NewShardingConsumer(shards int, consumer Consumer) *ShardingConsumer {
	if shards <= 0 {
		shards = 1
	}

	s := &ShardingConsumer{
		consumer: consumer,
		shards:   make([]chan shardingRequest, shards),
	}

	for i := range s.shards {
		s.shards[i] = make(chan shardingRequest)
		s.wg.Go(func() { s.runShard(s.shards[i]) })
	}

	return s
}

// ShardingConsumer shareds each stream to a shard based on the stream's labels.
type ShardingConsumer struct {
	shards []chan shardingRequest

	consumer Consumer
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// Consume shards each stream in the batch to a shard and waits for all to finish.
func (s *ShardingConsumer) Consume(ctx context.Context, batch Batch) error {
	streamLen := batch.StreamLen()
	if streamLen == 0 {
		return nil
	}

	errChans := make([]<-chan error, 0, streamLen)

	// NOTE: when we only have one stream in a batch there is no need to split it.
	if streamLen == 1 {
		errChans = append(errChans, s.consume(ctx, batch, batch.streams[0].Labels))
	} else {
		batch.ConsumeStreams(func(stream Stream, created int64) {
			streamBatch := NewBatchWithCreatedUnixMicro(created)
			streamBatch.Add(stream)
			errChans = append(errChans, s.consume(ctx, streamBatch, stream.Labels))
		})
	}

	return s.joinErrors(ctx, errChans)
}

func (s *ShardingConsumer) consume(ctx context.Context, batch Batch, lables model.LabelSet) <-chan error {
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
	case s.shards[s.shardFor(lables)] <- req:
	}

	return errCh
}

// ConsumeEntry shards a single entry to the shard selected by the entry's labels.
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
	case s.shards[s.shardFor(entry.Labels)] <- req:
	}

	return s.joinErrors(ctx, []<-chan error{errCh})
}

// Stop stops the consumer and waits for shards to exit.
func (s *ShardingConsumer) Stop() {
	s.stopOnce.Do(func() {
		for _, shard := range s.shards {
			close(shard)
		}
		s.wg.Wait()
	})
}

func (s *ShardingConsumer) shardFor(labels model.LabelSet) int {
	return int(labels.FastFingerprint() % model.Fingerprint(len(s.shards)))
}

func (s *ShardingConsumer) runShard(shard <-chan shardingRequest) {
	for req := range shard {
		req.errCh <- req.consume(s.consumer)
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
