package config_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec/config"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

// TODO: Add a test using the sending_queue > batch block.
// TODO: Add a test using the deprecated batcher block together with the sending_queue > batch block.
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
			var sl config.SplunkHecClientArguments
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
			name: "valid splunk config - nested blocks",
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
				heartbeat {
				   interval = "10s"
				   startup = true
				}
				telemetry {
				   enabled = true
		        }
				
				`,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl config.SplunkConf
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
			name: "invalid splunkhec arguments, no endpoint",
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
		{
			name: "valid splunkhec arguments, with hearthbeat config",
			cfg: `
			splunk {
				log_data_enabled = true
				profiling_data_enabled = true
				source = "def"
				token = "abc"
				heartbeat {
				   interval = "10s"
				   startup = true
	         	}
	     	}
	     	client  {
			    endpoint = "http://localhost:8088"
	     		timeout = "10s"
	     		insecure_skip_verify = true
	     		max_idle_conns = 10
		}
		`,
			expectErr: false,
		},
		{
			name: "valid splunkhec arguments, with otelattrs",
			cfg: `
			splunk {
				log_data_enabled = true
				profiling_data_enabled = true
				source = "def"
				token = "abc"
	     	}
	     	client  {
			    endpoint = "http://localhost:8088"
	     		timeout = "10s"
	     		insecure_skip_verify = true
	     		max_idle_conns = 10
			}
			otel_attrs_to_hec_metadata {
				source = "mysource"
				sourcetype = "mysourcetype"
				index = "myindex"
				host = "myhost"
			}
		`,
			expectErr: false,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl config.SplunkHecArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
