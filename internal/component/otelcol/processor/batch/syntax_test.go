package batch_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestDecodeDocumentedSyntax(t *testing.T) {
	var args batch.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		timeout = "5s"
		send_batch_size = 100
		send_batch_max_size = 200
		metadata_keys = ["a", "b"]
		metadata_cardinality_limit = 7
		output {}
		debug_metrics { disable_high_cardinality_metrics = false }
	`), &args))
	require.Equal(t, 5*time.Second, args.Timeout)
	require.Equal(t, uint32(200), args.SendBatchMaxSize)
	require.Equal(t, []string{"a", "b"}, args.MetadataKeys)
}
