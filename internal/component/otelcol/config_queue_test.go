package otelcol_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	otelexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestBatchConfig_ConvertPartition(t *testing.T) {
	args := &otelcol.BatchConfig{
		FlushTimeout: time.Second,
		MinSize:      10,
		Sizer:        "items",
		Partition: &otelcol.BatchPartitionConfig{
			MetadataKeys: []string{"tenant"},
		},
	}

	converted, err := args.Convert()
	require.NoError(t, err)
	require.True(t, converted.HasValue())
	require.Equal(t, otelexporterhelper.RequestSizerTypeItems, converted.Get().Sizer)
	require.Equal(t, []string{"tenant"}, converted.Get().Partition.MetadataKeys)
}

func TestQueueArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected otelcol.QueueArguments
		errorMsg string
	}{
		{
			testName: "default queue arguments",
			cfg:      ``,
			expected: otelcol.QueueArguments{
				Enabled:         true,
				NumConsumers:    10,
				QueueSize:       1000,
				BlockOnOverflow: false,
				Sizer:           "requests",
				WaitForResult:   false,
			},
		},
		{
			testName: "all arguments supplied without batch block",
			cfg: `
				enabled = true
				num_consumers = 5
				queue_size = 2000
				block_on_overflow = true
				sizer = "bytes"
				wait_for_result = true
			`,
			expected: otelcol.QueueArguments{
				Enabled:         true,
				NumConsumers:    5,
				QueueSize:       2000,
				BlockOnOverflow: true,
				Sizer:           "bytes",
				WaitForResult:   true,
			},
		},
		{
			testName: "all arguments supplied with batch block",
			cfg: `
				enabled = true
				num_consumers = 8
				queue_size = 1500
				block_on_overflow = false
				sizer = "items"
				wait_for_result = false

				batch {
					flush_timeout = "5s"
					min_size = 100
					max_size = 500
					sizer = "bytes"

					partition {
						metadata_keys = ["tenant", "cluster"]
					}
				}
			`,
			expected: otelcol.QueueArguments{
				Enabled:         true,
				NumConsumers:    8,
				QueueSize:       1500,
				BlockOnOverflow: false,
				Sizer:           "items",
				WaitForResult:   false,
				Batch: &otelcol.BatchConfig{
					FlushTimeout: 5 * time.Second,
					MinSize:      100,
					MaxSize:      500,
					Sizer:        "bytes",
					Partition: &otelcol.BatchPartitionConfig{
						MetadataKeys: []string{"tenant", "cluster"},
					},
				},
			},
		},
		{
			testName: "queue disabled",
			cfg: `
				enabled = false
				num_consumers = 5
				queue_size = 100
			`,
			expected: otelcol.QueueArguments{
				Enabled:         false,
				NumConsumers:    5,
				QueueSize:       100,
				BlockOnOverflow: false,
				Sizer:           "requests",
				WaitForResult:   false,
			},
		},
		{
			testName: "invalid sizer",
			cfg: `
				enabled = true
				sizer = "invalid_sizer"
			`,
			errorMsg: "invalid sizer: invalid_sizer",
		},
		{
			testName: "invalid queue size",
			cfg: `
				enabled = true
				queue_size = 0
			`,
			errorMsg: "queue_size must be greater than zero",
		},
		{
			testName: "batch min_size greater than queue_size",
			cfg: `
				enabled = true
				queue_size = 100
				sizer = "items"

				batch {
					flush_timeout = "5s"
					min_size = 200
					sizer = "items"
				}
			`,
			errorMsg: "`min_size` must be less than or equal to `queue_size`",
		},
		{
			testName: "batch with invalid sizer",
			cfg: `
				enabled = true

				batch {
					flush_timeout = "5s"
					min_size = 100
					sizer = "requests"
				}
			`,
			// Update the docs if the requests sizer becomes supported.
			errorMsg: "`batch` supports only `items` or `bytes` sizer",
		},
		{
			testName: "batch with invalid flush_timeout",
			cfg: `
				enabled = true

				batch {
					flush_timeout = "0s"
					min_size = 100
					sizer = "items"
				}
			`,
			errorMsg: "`flush_timeout` must be positive",
		},
		{
			testName: "batch with negative min_size",
			cfg: `
				enabled = true

				batch {
					flush_timeout = "5s"
					min_size = -1
					sizer = "items"
				}
			`,
			errorMsg: "`min_size` must be non-negative",
		},
		{
			testName: "batch with max_size less than min_size",
			cfg: `
				enabled = true

				batch {
					flush_timeout = "5s"
					min_size = 200
					max_size = 100
					sizer = "items"
				}
			`,
			errorMsg: "`max_size` must be greater or equal to `min_size`",
		},
		{
			testName: "batch partition with duplicate metadata keys",
			cfg: `
				enabled = true

				batch {
					partition {
						metadata_keys = ["tenant", "TENANT"]
					}
				}
			`,
			errorMsg: `duplicate entry in metadata_keys: "tenant" (case-insensitive)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args otelcol.QueueArguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, args)
		})
	}
}
