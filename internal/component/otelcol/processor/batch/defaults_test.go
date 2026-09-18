package batch_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/batchprocessor"
)

func TestDefaultArguments(t *testing.T) {
	var args batch.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &batchprocessor.Config{
		Timeout:                  200 * time.Millisecond,
		SendBatchSize:            2000,
		SendBatchMaxSize:         3000,
		MetadataCardinalityLimit: 1000,
	}, cfg)
}
