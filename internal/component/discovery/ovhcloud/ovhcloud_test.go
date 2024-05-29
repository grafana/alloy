package ovhcloud_test

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	prom_ovh "github.com/prometheus/prometheus/discovery/ovhcloud"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery/ovhcloud"
	"github.com/grafana/alloy/syntax"
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected *prom_ovh.SDConfig
		errorMsg string
	}{
		{
			testName: "defaults",
			cfg: `
				application_key = "appkey"
				application_secret = "appsecret"
				consumer_key = "consumerkey"
				service = "dedicated_server"
			`,
			expected: &prom_ovh.SDConfig{
				Endpoint:          ovhcloud.DefaultArguments.Endpoint,
				ApplicationKey:    "appkey",
				ApplicationSecret: "appsecret",
				ConsumerKey:       "consumerkey",
				RefreshInterval:   model.Duration(ovhcloud.DefaultArguments.RefreshInterval),
				Service:           "dedicated_server",
			},
		},
		{
			testName: "explicit",
			cfg: `
				endpoint = "custom-endpoint"
				refresh_interval = "11m"
				application_key = "appkey"
				application_secret = "appsecret"
				consumer_key = "consumerkey"
				service = "vps"
			`,
			expected: &prom_ovh.SDConfig{
				Endpoint:          "custom-endpoint",
				ApplicationKey:    "appkey",
				ApplicationSecret: "appsecret",
				ConsumerKey:       "consumerkey",
				RefreshInterval:   model.Duration(11 * time.Minute),
				Service:           "vps",
			},
		},
		{
			testName: "empty application key",
			cfg: `
				endpoint = "custom-endpoint"
				refresh_interval = "11m"
				application_key = ""
				application_secret = "appsecret"
				consumer_key = "consumerkey"
				service = "vps"
			`,
			errorMsg: "application_key cannot be empty",
		},
		{
			testName: "empty application secret",
			cfg: `
				endpoint = "custom-endpoint"
				refresh_interval = "11m"
				application_key = "appkey"
				application_secret = ""
				consumer_key = "consumerkey"
				service = "vps"
			`,
			errorMsg: "application_secret cannot be empty",
		},
		{
			testName: "empty consumer key",
			cfg: `
				endpoint = "custom-endpoint"
				refresh_interval = "11m"
				application_key = "appkey"
				application_secret = "appsecret"
				consumer_key = ""
				service = "vps"
			`,
			errorMsg: "consumer_key cannot be empty",
		},
		{
			testName: "empty endpoint",
			cfg: `
				endpoint = ""
				refresh_interval = "11m"
				application_key = "appkey"
				application_secret = "appsecret"
				consumer_key = "consumerkey"
				service = "vps"
			`,
			errorMsg: "endpoint cannot be empty",
		},
		{
			testName: "unknown service",
			cfg: `
				endpoint = "custom-endpoint"
				refresh_interval = "11m"
				application_key = "appkey"
				application_secret = "appsecret"
				consumer_key = "consumerkey"
				service = "asdf"
			`,
			errorMsg: "unknown service: asdf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args ovhcloud.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}

			require.NoError(t, err)

			promArgs := args.Convert()

			require.Equal(t, tc.expected, promArgs)
		})
	}
}
