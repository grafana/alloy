package servicegraph_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/connector/servicegraph"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T {
	return &v
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected servicegraphconnector.Config
		errorMsg string
	}{
		{
			testName: "Defaults",
			cfg: `
				output {}
			`,
			expected: servicegraphconnector.Config{
				LatencyHistogramBuckets: []time.Duration{
					2 * time.Millisecond,
					4 * time.Millisecond,
					6 * time.Millisecond,
					8 * time.Millisecond,
					10 * time.Millisecond,
					50 * time.Millisecond,
					100 * time.Millisecond,
					200 * time.Millisecond,
					400 * time.Millisecond,
					800 * time.Millisecond,
					1 * time.Second,
					1400 * time.Millisecond,
					2 * time.Second,
					5 * time.Second,
					10 * time.Second,
					15 * time.Second,
				},
				Dimensions: []string{},
				Store: servicegraphconnector.StoreConfig{
					MaxItems: 1000,
					TTL:      2 * time.Second,
				},
				CacheLoop:                 1 * time.Minute,
				StoreExpirationLoop:       2 * time.Second,
				VirtualNodePeerAttributes: []string{"peer.service", "db.name", "db.system"},
				VirtualNodeExtraLabel:     false,
				DatabaseNameAttributes:    []string{"db.name"},
				MetricsFlushInterval:      ptr(60 * time.Second),
			},
		},
		{
			testName: "ExplicitValues",
			cfg: `
					dimensions = ["foo", "bar"]
					latency_histogram_buckets = ["2ms", "4s", "6h"]
					store {
						max_items = 333
						ttl = "12h"
					}
					cache_loop = "55m"
					store_expiration_loop = "77s"
					virtual_node_peer_attributes = ["attr1", "attr2"]
					virtual_node_extra_label = true
					metrics_flush_interval = "5s"
					exponential_histogram_max_size = 160
					output {}
				`,
			expected: servicegraphconnector.Config{
				LatencyHistogramBuckets: []time.Duration{
					2 * time.Millisecond,
					4 * time.Second,
					6 * time.Hour,
				},
				Dimensions: []string{"foo", "bar"},
				Store: servicegraphconnector.StoreConfig{
					MaxItems: 333,
					TTL:      12 * time.Hour,
				},
				CacheLoop:                   55 * time.Minute,
				StoreExpirationLoop:         77 * time.Second,
				VirtualNodePeerAttributes:   []string{"attr1", "attr2"},
				VirtualNodeExtraLabel:       true,
				DatabaseNameAttributes:      []string{"db.name"},
				MetricsFlushInterval:        ptr(5 * time.Second),
				ExponentialHistogramMaxSize: 160,
			},
		},
		{
			testName: "InvalidCacheLoop",
			cfg: `
					cache_loop = "0s"
					output {}
				`,
			errorMsg: "cache_loop must be greater than 0",
		},
		{
			testName: "InvalidStoreExpirationLoop",
			cfg: `
					store_expiration_loop = "0s"
					output {}
				`,
			errorMsg: "store_expiration_loop must be greater than 0",
		},
		{
			testName: "InvalidStoreMaxItems",
			cfg: `
					store {
						max_items = 0
					}

					output {}
				`,
			errorMsg: "store.max_items must be greater than 0",
		},
		{
			testName: "InvalidStoreTTL",
			cfg: `
					store {
						ttl = "0s"
					}

					output {}
				`,
			errorMsg: "store.ttl must be greater than 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args servicegraph.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}

			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*servicegraphconnector.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}
