package otelcol_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/stretchr/testify/require"
)

// Canaries for the shared blocks' defaults, several of which our docs promise once under
// docs/sources/shared/reference/components/ rather than per component. If one fails,
// someone changed a shared default: update the shared docs page, then these values.

func TestQueueArguments_Defaults(t *testing.T) {
	// docs: shared/reference/components/otelcol-queue-block.md
	var args otelcol.QueueArguments
	args.SetToDefault()

	require.Equal(t, otelcol.QueueArguments{
		Enabled:      true,
		NumConsumers: 10,
		QueueSize:    1000,
		Sizer:        "requests",
	}, args)
}

func TestBatchConfig_Defaults(t *testing.T) {
	// docs: shared/reference/components/otelcol-queue-batch-block.md
	//
	// These apply only when a batch block is written. QueueArguments leaves Batch nil,
	// so omitting the block keeps whatever the upstream exporter default is instead.
	var args otelcol.BatchConfig
	args.SetToDefault()

	require.Equal(t, otelcol.BatchConfig{
		FlushTimeout: 200 * time.Millisecond,
		MinSize:      2000,
		MaxSize:      3000,
		Sizer:        "items",
	}, args)
}

func TestRetryArguments_Defaults(t *testing.T) {
	// docs: shared/reference/components/otelcol-retry-block.md
	var args otelcol.RetryArguments
	args.SetToDefault()

	require.Equal(t, otelcol.RetryArguments{
		Enabled:             true,
		InitialInterval:     5 * time.Second,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         30 * time.Second,
		MaxElapsedTime:      5 * time.Minute,
	}, args)
}

func TestKafkaMetadataArguments_Defaults(t *testing.T) {
	// docs: shared/reference/components/otelcol-kafka-metadata.md and
	// shared/reference/components/otelcol-kafka-metadata-retry.md
	var args otelcol.KafkaMetadataArguments
	args.SetToDefault()

	require.Equal(t, otelcol.KafkaMetadataArguments{
		Full:            true,
		RefreshInterval: 10 * time.Minute,
		Retry: otelcol.KafkaMetadataRetryArguments{
			MaxRetries: 3,
			Backoff:    250 * time.Millisecond,
		},
	}, args)
}

// The three below have no single shared docs page; each consuming component documents
// its own values. They are pinned as regression cover for the shared default itself.

func TestControllerArguments_Defaults(t *testing.T) {
	var args otelcol.ControllerArguments
	args.SetToDefault()

	require.Equal(t, otelcol.ControllerArguments{
		CollectionInterval: time.Minute,
		InitialDelay:       time.Second,
	}, args)
}

func TestScraperControllerArguments_Defaults(t *testing.T) {
	var args otelcol.ScraperControllerArguments
	args.SetToDefault()

	require.Equal(t, otelcol.ScraperControllerArguments{
		CollectionInterval: time.Minute,
		InitialDelay:       time.Second,
	}, args)
}

func TestConsumerRetryArguments_Defaults(t *testing.T) {
	var args otelcol.ConsumerRetryArguments
	args.SetToDefault()

	require.Equal(t, otelcol.ConsumerRetryArguments{
		Enabled:         false,
		InitialInterval: time.Second,
		MaxInterval:     30 * time.Second,
		MaxElapsedTime:  5 * time.Minute,
	}, args)
}
