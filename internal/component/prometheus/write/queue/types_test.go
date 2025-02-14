package queue

import (
	"testing"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestParsingTLSConfig(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(`
    endpoint "cloud"  {
        url = "http://example.com"
        basic_auth {
            username = 12345
            password = "password"
        }
        tls_config {
            insecure_skip_verify = true
        }

    }
`), &args)

	require.NoError(t, err)
}

func TestParralelismConfig_Validate(t *testing.T) {
	testCases := []struct {
		name           string
		config         func(cfg ParallelismConfig) ParallelismConfig
		expectedErrMsg string
	}{
		{
			name: "default config is valid",
			config: func(cfg ParallelismConfig) ParallelismConfig {
				return cfg
			},
		},
		{
			name: "positive drift scale up seconds is invalid",
			config: func(cfg ParallelismConfig) ParallelismConfig {
				cfg.DriftScaleUp = 10 * time.Second
				cfg.DriftScaleDown = 10 * time.Second
				return cfg
			},
			expectedErrMsg: "drift_scale_up_seconds less than or equal drift_scale_down_seconds",
		},
		{
			name: "max less than min",
			config: func(cfg ParallelismConfig) ParallelismConfig {
				cfg.MaxConnections = 1
				cfg.MinConnections = 2
				return cfg
			},
			expectedErrMsg: "max_connections less than min_connections",
		},
		{
			name: "to low desired check",
			config: func(cfg ParallelismConfig) ParallelismConfig {
				cfg.DesiredCheckInterval = (1 * time.Second) - (50 * time.Millisecond)
				return cfg
			},
			expectedErrMsg: "desired_check_interval must be greater than or equal to 1 second",
		},
		{
			name: "invalid network error percentage low",
			config: func(cfg ParallelismConfig) ParallelismConfig {
				cfg.AllowedNetworkErrorFraction = -0.01
				return cfg
			},
			expectedErrMsg: "allowed_network_error_percent must be between 0.00 and 1.00",
		},
		{
			name: "invalid network error percentage high",
			config: func(cfg ParallelismConfig) ParallelismConfig {
				cfg.AllowedNetworkErrorFraction = 1.01
				return cfg
			},
			expectedErrMsg: "allowed_network_error_percent must be between 0.00 and 1.00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultEndpointConfig()
			cfg.Parallelism = tc.config(cfg.Parallelism)
			args := &Arguments{
				Endpoints: []EndpointConfig{cfg},
			}
			err := args.Validate()

			if tc.expectedErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrMsg)
			}
		})
	}
}
