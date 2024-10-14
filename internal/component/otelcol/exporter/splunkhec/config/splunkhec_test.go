package splunkhec_config_test

import (
	"testing"

	splunkhec_config "github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec/config"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalSplunkHecClientArguments(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid client config",
			cfg: `
			   endpoint = "http://localhost:8088"
			   timeout = "10s"
			   insecure_skip_verify = true
			   max_idle_conns = 10
			`,
		},
		{
			name: "invalid client config",
			cfg: `
			   timeout = "10s"
			   insecure_skip_verify = true
			   max_idle_conns = 10
			   `,
			expectErr: true,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl splunkhec_config.SplunkHecClientArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalSplunkConf(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid splunk config",
			cfg: `
				log_data_enabled = true
				profiling_data_enabled = true
				token = "abc"
				source = "def"
				sourcetype = "ghi"
				index = "jkl"
				disable_compression = true
				max_content_length_logs = 100
				max_content_length_metrics = 200
				max_content_length_traces = 300
				max_event_size = 400
			`,
		},
		{
			name: "invalid splunk config - no token",
			cfg: `
				log_data_enabled = true
				profiling_data_enabled = true
				source = "def"
				sourcetype = "ghi"
				index = "jkl"
				disable_compression = true
				max_content_length_logs = 100
				max_content_length_metrics = 200
				max_content_length_traces = 300
				max_event_size = 400
				`,
			expectErr: true,
		},
		{
			name: "valid splunk config - no index",
			cfg: `
				log_data_enabled = true
				profiling_data_enabled = true
				source = "def"
				sourcetype = "ghi"
				token = "jkl"
				disable_compression = true
				max_content_length_logs = 100
				max_content_length_metrics = 200
				max_content_length_traces = 300
				max_event_size = 400
				`,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl splunkhec_config.SplunkConf
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalSplunkHecArguments(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid splunkhec arguments",
			cfg: `
			splunk {
				log_data_enabled = true
				profiling_data_enabled = true
				token = "abc"
				source = "def"
	     	}
	     	client  {
	     		endpoint = "http://localhost:8088"
	     		timeout = "10s"
	     		insecure_skip_verify = true
	     		max_idle_conns = 10
		}
		`,
		}, {
			name: "invalidvalid splunkhec arguments, no token",
			cfg: `
			splunk {
				log_data_enabled = true
				profiling_data_enabled = true
				source = "def"
	     	}
	     	client  {
	     		endpoint = "http://localhost:8088"
	     		timeout = "10s"
	     		insecure_skip_verify = true
	     		max_idle_conns = 10
		}
		`,
			expectErr: true,
		},
		{
			name: "invalidvalid splunkhec arguments, no endpoint",
			cfg: `
			splunk {
				log_data_enabled = true
				profiling_data_enabled = true
				source = "def"
				token = "abc"
	     	}
	     	client  {
	     		timeout = "10s"
	     		insecure_skip_verify = true
	     		max_idle_conns = 10
		}
		`,
			expectErr: true,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl splunkhec_config.SplunkHecArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

}
