package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParralelismConfig_Validate(t *testing.T) {
	testCases := []struct {
		name           string
		config         func(cfg ParralelismConfig) ParralelismConfig
		expectedErrMsg string
	}{
		{
			name: "default config is valid",
			config: func(cfg ParralelismConfig) ParralelismConfig {
				return cfg
			},
		},
		{
			name: "positive drift scale up seconds is invalid",
			config: func(cfg ParralelismConfig) ParralelismConfig {
				cfg.DriftScaleUpSeconds = 10
				cfg.DriftScaleDownSeconds = 10
				return cfg
			},
			expectedErrMsg: "drift_scale_up_seconds less than or equal drift_scale_down_seconds",
		},
		{
			name: "max less than min",
			config: func(cfg ParralelismConfig) ParralelismConfig {
				cfg.MaxConnections = 1
				cfg.MinConnections = 2
				return cfg
			},
			expectedErrMsg: "max_connections less than min_connections",
		},
		{
			name: "to low desired check",
			config: func(cfg ParralelismConfig) ParralelismConfig {
				cfg.DesiredCheckInterval = (1 * time.Second) - (50 * time.Millisecond)
				return cfg
			},
			expectedErrMsg: "desired_check_interval must be greater than or equal to 1 second",
		},
		{
			name: "invalid network error percentage low",
			config: func(cfg ParralelismConfig) ParralelismConfig {
				cfg.AllowedNetworkErrorPercent = -0.01
				return cfg
			},
			expectedErrMsg: "allowed_network_error_percent must be between 0.00 and 1.00",
		},
		{
			name: "invalid network error percentage high",
			config: func(cfg ParralelismConfig) ParralelismConfig {
				cfg.AllowedNetworkErrorPercent = 1.01
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
