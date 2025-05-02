package splunkhec_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigConversion(t *testing.T) {
	expectedCustomise := splunkhecexporter.Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8088", ProxyURL: "",
			TLSSetting: configtls.ClientConfig{
				Config: configtls.Config{
					CAFile:                   "",
					CAPem:                    "",
					IncludeSystemCACertsPool: false,
					CertFile:                 "",
					CertPem:                  "",
					KeyFile:                  "",
					KeyPem:                   "",
					MinVersion:               "",
					MaxVersion:               "",
					CipherSuites:             []string(nil), ReloadInterval: 0,
				},
				Insecure:           false,
				InsecureSkipVerify: true,
				ServerName:         "",
			},
			ReadBufferSize:       0,
			WriteBufferSize:      0,
			Timeout:              10000000000,
			Headers:              map[string]configopaque.String(nil),
			Auth:                 (*configauth.Authentication)(nil),
			Compression:          "",
			MaxIdleConns:         100,
			MaxIdleConnsPerHost:  0,
			MaxConnsPerHost:      0,
			IdleConnTimeout:      90000000000,
			DisableKeepAlives:    false,
			HTTP2ReadIdleTimeout: 0,
			HTTP2PingTimeout:     0,
			Cookies:              (*confighttp.CookiesConfig)(nil),
		},
		QueueSettings: exporterhelper.QueueConfig{
			Enabled:      true,
			NumConsumers: 10,
			QueueSize:    1000,
			StorageID:    nil,
		},
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5000000000,
			RandomizationFactor: 0.5,
			Multiplier:          1.5,
			MaxInterval:         30000000000,
			MaxElapsedTime:      300000000000,
		},
		BatcherConfig: exporterhelper.BatcherConfig{
			Enabled:      false,
			FlushTimeout: 200000000,
			SizeConfig: exporterhelper.SizeConfig{
				MinSize: 8192,
				MaxSize: 0,
				Sizer: func() exporterhelper.RequestSizerType {
					var s exporterhelper.RequestSizerType
					require.NoError(t, s.UnmarshalText([]byte("items")))
					return s
				}(),
			},
		},
		LogDataEnabled:          true,
		ProfilingDataEnabled:    true,
		Token:                   "token",
		Source:                  "source",
		SourceType:              "sourcetype",
		Index:                   "index",
		DisableCompression:      false,
		MaxContentLengthLogs:    0x200000,
		MaxContentLengthMetrics: 0x200000,
		MaxContentLengthTraces:  0x200000,
		MaxEventSize:            0x500000,
		SplunkAppName:           "Alloy",
		SplunkAppVersion:        "",
		HecFields:               splunkhecexporter.OtelToHecFields{SeverityText: "", SeverityNumber: ""},
		HealthPath:              "/services/collector/health",
		HecHealthCheckEnabled:   false,
		ExportRaw:               false,
		UseMultiMetricFormat:    false,
		Heartbeat:               splunkhecexporter.HecHeartbeat{Interval: 0, Startup: false},
		Telemetry: splunkhecexporter.HecTelemetry{
			Enabled:              false,
			OverrideMetricsNames: map[string]string(nil),
			ExtraAttributes:      map[string]string(nil),
		},
	}

	expectedMinimal := &splunkhecexporter.Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8088",
			ProxyURL: "",
			TLSSetting: configtls.ClientConfig{
				Config: configtls.Config{
					CAFile:                   "",
					CAPem:                    "",
					IncludeSystemCACertsPool: false,
					CertFile:                 "",
					CertPem:                  "",
					KeyFile:                  "",
					KeyPem:                   "",
					MinVersion:               "",
					MaxVersion:               "",
					CipherSuites:             []string(nil),
					ReloadInterval:           0,
				},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			}, ReadBufferSize: 0,
			WriteBufferSize:      0,
			Timeout:              15000000000,
			Headers:              map[string]configopaque.String(nil),
			Auth:                 (*configauth.Authentication)(nil),
			Compression:          "",
			MaxIdleConns:         100,
			MaxIdleConnsPerHost:  0,
			MaxConnsPerHost:      0,
			IdleConnTimeout:      90000000000,
			DisableKeepAlives:    false,
			HTTP2ReadIdleTimeout: 0,
			HTTP2PingTimeout:     0,
			Cookies:              (*confighttp.CookiesConfig)(nil)},
		QueueSettings: exporterhelper.QueueConfig{
			Enabled:      true,
			NumConsumers: 10,
			QueueSize:    1000,
			StorageID:    (nil),
		},
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5000000000,
			RandomizationFactor: 0.5,
			Multiplier:          1.5,
			MaxInterval:         30000000000,
			MaxElapsedTime:      300000000000,
		},
		BatcherConfig: exporterhelper.BatcherConfig{
			Enabled:      false,
			FlushTimeout: 200000000,
			SizeConfig: exporterhelper.SizeConfig{
				MinSize: 8192,
				MaxSize: 0,
				Sizer: func() exporterhelper.RequestSizerType {
					var s exporterhelper.RequestSizerType
					require.NoError(t, s.UnmarshalText([]byte("items")))
					return s
				}(),
			},
		},
		LogDataEnabled:       true,
		ProfilingDataEnabled: true,
		Token:                "token", Source: "",
		SourceType: "", Index: "",
		DisableCompression:      false,
		MaxContentLengthLogs:    0x200000,
		MaxContentLengthMetrics: 0x200000,
		MaxContentLengthTraces:  0x200000,
		MaxEventSize:            0x500000,
		SplunkAppName:           "Alloy",
		HecFields:               splunkhecexporter.OtelToHecFields{SeverityText: "", SeverityNumber: ""},
		HealthPath:              "/services/collector/health", HecHealthCheckEnabled: false,
		ExportRaw:            false,
		UseMultiMetricFormat: false,
		Heartbeat:            splunkhecexporter.HecHeartbeat{Interval: 0, Startup: false},
		Telemetry: splunkhecexporter.HecTelemetry{
			Enabled:              false,
			OverrideMetricsNames: map[string]string(nil),
			ExtraAttributes:      map[string]string(nil),
		}}

	tests := []struct {
		testName string
		alloyCfg string
		expected *splunkhecexporter.Config
	}{
		{
			testName: "full customise",
			alloyCfg: `
				splunk {
				   token = "token"
				   source = "source"
				   sourcetype = "sourcetype"
				   index = "index"
				}
				client {
				   endpoint = "http://localhost:8088"
				   timeout = "10s"
				   insecure_skip_verify = true
		        }
			`,
			expected: &expectedCustomise,
		},
		{
			testName: "minimal customise",
			alloyCfg: `
				splunk {
				   token = "token"
		         }
				client {
				  endpoint = "http://localhost:8088"
				}
				`,
			expected: expectedMinimal,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			var args splunkhec.Arguments
			err := syntax.Unmarshal([]byte(tt.alloyCfg), &args)
			if err != nil {
				t.Fatal(err)
			}

			cfg, err := args.Convert()
			if err != nil {
				t.Fatal(err)
			}

			require.Equal(t, tt.expected, cfg)
		})
	}
}
