package write

import (
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
	"github.com/grafana/alloy/syntax"
)

func TestUnmarshalAlloy(t *testing.T) {
	type testCase struct {
		name      string
		cfg       string
		expectErr bool
		expected  Arguments
	}

	tests := []testCase{
		{
			name:      "empty config",
			cfg:       "",
			expectErr: true,
		},
		{
			name: "duplicate endpoint names",
			cfg: `
			endpoint {
				name = "test"
				url  = "http://localhost:3100/loki/api/v1/push"
			}
			endpoint {
				name = "test"
				url  = "http://localhost:3200/loki/api/v1/push"
			}
			`,
			expectErr: true,
		},
		{
			name: "single endpoint",
			cfg: `
			endpoint {
				name           = "test-url"
				url            = "http://0.0.0.0:11111/loki/api/v1/push"
				remote_timeout = "100ms"
			}
			`,
			expected: Arguments{
				Endpoints: []EndpointArguments{
					{
						Name:              "test-url",
						URL:               "http://0.0.0.0:11111/loki/api/v1/push",
						BatchWait:         1 * time.Second,
						BatchSize:         1 * units.MiB,
						RemoteTimeout:     100 * time.Millisecond,
						MinBackoff:        500 * time.Millisecond,
						MaxBackoff:        5 * time.Minute,
						MaxBackoffRetries: 10,
						RetryOnHTTP429:    true,
						HTTPClientConfig:  config.CloneDefaultHTTPClientConfig(),
						QueueConfig:       defaultQueueConfigArguments,
					},
				},
			},
		},
		{
			name: "multiple auth methods",
			cfg: `
			endpoint {
				url               = "http://localhost:3100/loki/api/v1/push"
				bearer_token      = "token"
				bearer_token_file = "/path/to/file.token"
			}
			`,
			expectErr: true,
		},
		{
			name: "wal min read frequency higher than max",
			cfg: `
			endpoint {
				url = "http://localhost:3100/loki/api/v1/push"
			}
			wal {
				enabled            = true
				min_read_frequency = "1h"
				max_read_frequency = "1m"
			}
			`,
			expectErr: true,
		},
		{
			name: "wal disabled by default",
			cfg: `
			endpoint {
				url = "http://localhost:3100/loki/api/v1/push"
			}
			wal {}
			`,
			expected: Arguments{
				Endpoints: []EndpointArguments{
					{
						URL:               "http://localhost:3100/loki/api/v1/push",
						BatchWait:         1 * time.Second,
						BatchSize:         1 * units.MiB,
						RemoteTimeout:     10 * time.Second,
						MinBackoff:        500 * time.Millisecond,
						MaxBackoff:        5 * time.Minute,
						MaxBackoffRetries: 10,
						RetryOnHTTP429:    true,
						HTTPClientConfig:  config.CloneDefaultHTTPClientConfig(),
						QueueConfig:       defaultQueueConfigArguments,
					},
				},
				WAL: WalArguments{
					Enabled:          false,
					MaxSegmentAge:    wal.DefaultMaxSegmentAge,
					MinReadFrequency: wal.DefaultWatchConfig.MinReadFrequency,
					MaxReadFrequency: wal.DefaultWatchConfig.MaxReadFrequency,
					DrainTimeout:     wal.DefaultWatchConfig.DrainTimeout,
				},
			},
		},
		{
			name: "wal enabled with defaults",
			cfg: `
			endpoint {
				url = "http://localhost:3100/loki/api/v1/push"
			}
			wal {
				enabled = true
			}
			`,
			expected: Arguments{
				Endpoints: []EndpointArguments{
					{
						URL:               "http://localhost:3100/loki/api/v1/push",
						BatchWait:         1 * time.Second,
						BatchSize:         1 * units.MiB,
						RemoteTimeout:     10 * time.Second,
						MinBackoff:        500 * time.Millisecond,
						MaxBackoff:        5 * time.Minute,
						MaxBackoffRetries: 10,
						RetryOnHTTP429:    true,
						HTTPClientConfig:  config.CloneDefaultHTTPClientConfig(),
						QueueConfig:       defaultQueueConfigArguments,
					},
				},
				WAL: WalArguments{
					Enabled:          true,
					MaxSegmentAge:    wal.DefaultMaxSegmentAge,
					MinReadFrequency: wal.DefaultWatchConfig.MinReadFrequency,
					MaxReadFrequency: wal.DefaultWatchConfig.MaxReadFrequency,
					DrainTimeout:     wal.DefaultWatchConfig.DrainTimeout,
				},
			},
		},
		{
			name: "wal enabled with some overrides",
			cfg: `
			endpoint {
				url = "http://localhost:3100/loki/api/v1/push"
			}
			wal {
				enabled            = true
				max_segment_age    = "10m"
				min_read_frequency = "11ms"
				drain_timeout      = "5m"
			}
			`,
			expected: Arguments{
				Endpoints: []EndpointArguments{
					{
						URL:               "http://localhost:3100/loki/api/v1/push",
						BatchWait:         1 * time.Second,
						BatchSize:         1 * units.MiB,
						RemoteTimeout:     10 * time.Second,
						MinBackoff:        500 * time.Millisecond,
						MaxBackoff:        5 * time.Minute,
						MaxBackoffRetries: 10,
						RetryOnHTTP429:    true,
						HTTPClientConfig:  config.CloneDefaultHTTPClientConfig(),
						QueueConfig:       defaultQueueConfigArguments,
					},
				},
				WAL: WalArguments{
					Enabled:          true,
					MaxSegmentAge:    10 * time.Minute,
					MinReadFrequency: 11 * time.Millisecond,
					MaxReadFrequency: wal.DefaultWatchConfig.MaxReadFrequency,
					DrainTimeout:     5 * time.Minute,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tt.cfg), &args)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, args)
		})
	}
}
