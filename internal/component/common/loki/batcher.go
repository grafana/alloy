package loki

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"golang.org/x/time/rate"
)

// StreamBatcher collects entries of a single stream from one producer and
// delivers them on Flush. A StreamBatcher can be reused after
// Flush and is not safe for concurrent use.
type StreamBatcher interface {
	// Add adds an entry to the stream and reports whether the caller must
	// Flush before the next Add.
	Add(entry push.Entry) bool
	// Flush delivers all entries added since the last Flush. A V2 batcher
	// retries while a consumer has stopped, and entries are dropped only when ctx
	// is canceled or the consumer returns an unexpected error. Dropped entries
	// may have reached some targets, and a later Flush does not resend them.
	Flush(ctx context.Context) error
}

var _ StreamBatcher = (*StreamBatcherV1)(nil)

// StreamBatcherV1 holds the last added entry and sends it to a LogsReceiver
// on Flush.
type StreamBatcherV1 struct {
	recv    LogsReceiver
	labels  model.LabelSet
	entry   push.Entry
	pending bool
}

// NewStreamBatcherV1 creates a StreamBatcherV1 that sends entries with labels
// to recv.
func NewStreamBatcherV1(recv LogsReceiver, labels model.LabelSet) *StreamBatcherV1 {
	return &StreamBatcherV1{recv: recv, labels: labels}
}

func (b *StreamBatcherV1) Add(entry push.Entry) bool {
	b.entry = entry
	b.pending = true
	return true
}

func (b *StreamBatcherV1) Flush(ctx context.Context) error {
	if !b.pending {
		return nil
	}

	b.pending = false
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.recv.Chan() <- NewEntry(b.labels, b.entry):
		return nil
	}
}

var (
	// consumerBackoffConfig is how long StreamBatcherV2 waits between attempts when the consumer has
	// stopped. It never gives up, the retries only end when the context is canceled.
	consumerBackoffConfig = backoff.Config{
		MinBackoff: 1 * time.Millisecond,
		MaxBackoff: 1 * time.Second,
	}
	// retryWarning lets a warning about retrying through at most once every 10 seconds.
	// It is shared by all batchers so a stalled pipeline does not flood the logs.
	retryWarning = rate.NewLimiter(rate.Every(10*time.Second), 1)
)

var _ StreamBatcher = (*StreamBatcherV2)(nil)

// StreamBatcherV2 collects entries into one stream and passes it to a
// Consumer on Flush. Flush retries while the consumer has stopped, until the
// consumer accepts the batch or the context is canceled.
type StreamBatcherV2 struct {
	consumer  Consumer
	labels    model.LabelSet
	flushSize int
	logger    *slog.Logger

	created int64
	batch   Batch
	entries []push.Entry
	attempt []push.Entry
}

// NewStreamBatcherV2 creates a StreamBatcherV2 that collects entries
// with labels and passes them to consumer on Flush. Add asks for a Flush
// once flushSize entries are added.
func NewStreamBatcherV2(logger *slog.Logger, consumer Consumer, labels model.LabelSet, flushSize int) *StreamBatcherV2 {
	return &StreamBatcherV2{
		consumer:  consumer,
		labels:    labels,
		flushSize: flushSize,
		logger:    logger,
	}
}

func (b *StreamBatcherV2) Add(entry push.Entry) bool {
	if len(b.entries) == 0 {
		b.created = time.Now().UnixMicro()
	}
	b.entries = append(b.entries, entry)
	return len(b.entries) >= b.flushSize
}

func (b *StreamBatcherV2) Flush(ctx context.Context) error {
	if len(b.entries) == 0 {
		return nil
	}

	defer func() {
		b.batch.Reset()
		// We need to clear and reset both entries and attempt, otherwise they keep the last flush's
		// lines and metadata alive while idle.
		clear(b.entries)
		b.entries = b.entries[:0]
		clear(b.attempt)
		b.attempt = b.attempt[:0]
	}()

	var retry *backoff.Backoff
	for {
		// Rebuild the batch from a copy of the entries on every attempt, since downstream may have modified it.
		b.batch.Reset()
		b.attempt = cloneEntriesInto(b.attempt, b.entries)
		b.batch.Add(NewStreamWithCreatedUnixMicro(b.labels, b.created, b.attempt...))

		// The error handling must follow the Consume contract, including the order of the checks.
		err := b.consumer.Consume(ctx, b.batch)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !errors.Is(err, ErrConsumerStopped) {
			return fmt.Errorf("unexpected error from consumer, this is most likely a bug: %w", err)
		}

		if retry == nil {
			retry = backoff.New(ctx, consumerBackoffConfig)
		}

		if retryWarning.Allow() {
			b.logger.Warn("failed to send entries, retrying until they are accepted", "labels", b.labels, "err", err, "retries", retry.NumRetries())
		}

		retry.Wait()
		if !retry.Ongoing() {
			return retry.Err()
		}
	}
}

// cloneEntriesInto replaces the content of dst with a copy of src and returns it.
func cloneEntriesInto(dst, src []push.Entry) []push.Entry {
	dst = dst[:0]
	for _, entry := range src {
		// Components can modify the structured metadata directly in place. Without its own copy
		// that would also change src, which we may have to send again. Cloning a nil slice
		// does not allocate, so entries without structured metadata cost nothing extra.
		entry.StructuredMetadata = slices.Clone(entry.StructuredMetadata)
		dst = append(dst, entry)
	}
	return dst
}
