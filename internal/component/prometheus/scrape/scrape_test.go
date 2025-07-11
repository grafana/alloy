package scrape

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/ckit/memconn"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	component_config "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	targets         = [{ "target1" = "target1" }]
	forward_to      = []
	scrape_interval = "10s"
	job_name        = "local"
	track_timestamps_staleness = true

	bearer_token = "token"
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	scrape_failure_log_file = "/path/to/file.log"

	tls_config {
		ca_file = "/path/to/file.ca"
		cert_file = "/path/to/file.cert"
		key_file = "/path/to/file.key"
		server_name = "server_name"
		insecure_skip_verify = false
		min_version = "TLS13"
	}

	http_headers = {
		"foo" = ["foobar"],
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestDefaults(t *testing.T) {
	var args Arguments
	args.SetToDefault()
	require.NoError(t, args.Validate())

	require.Equal(t, "/metrics", args.MetricsPath)
	require.Equal(t, "http", args.Scheme)
	require.Equal(t, false, args.HonorLabels)
	require.Equal(t, true, args.HonorTimestamps)
	require.Equal(t, false, args.TrackTimestampsStaleness)
	require.Equal(t, component_config.DefaultHTTPClientConfig, args.HTTPClientConfig)
	require.Equal(t, time.Minute, args.ScrapeInterval)
	require.Equal(t, time.Second*10, args.ScrapeTimeout)
	require.Equal(t, []string{
		"OpenMetricsText1.0.0",
		"OpenMetricsText0.0.1",
		"PrometheusText1.0.0",
		"PrometheusText0.0.4",
	}, args.ScrapeProtocols)
}

func TestDefaultsWithNativeHistograms(t *testing.T) {
	var args Arguments
	args.SetToDefault()
	args.EnableProtobufNegotiation = true
	require.NoError(t, args.Validate())

	require.Equal(t, "/metrics", args.MetricsPath)
	require.Equal(t, "http", args.Scheme)
	require.Equal(t, false, args.HonorLabels)
	require.Equal(t, true, args.HonorTimestamps)
	require.Equal(t, false, args.TrackTimestampsStaleness)
	require.Equal(t, component_config.DefaultHTTPClientConfig, args.HTTPClientConfig)
	require.Equal(t, time.Minute, args.ScrapeInterval)
	require.Equal(t, time.Second*10, args.ScrapeTimeout)
	require.Equal(t, []string{
		"PrometheusProto",
		"OpenMetricsText1.0.0",
		"OpenMetricsText0.0.1",
		"PrometheusText1.0.0",
		"PrometheusText0.0.4",
	}, args.ScrapeProtocols)
}

func TestBadAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	targets         = [{ "target1" = "target1" }]
	forward_to      = []
	scrape_interval = "10s"
	job_name        = "local"

	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}

func TestForwardingToAppendable(t *testing.T) {
	opts := component.Options{
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus_client.NewRegistry(),
		GetServiceData: func(name string) (interface{}, error) {
			switch name {
			case http_service.ServiceName:
				return http_service.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "alloy.internal:1245",
					BaseHTTPPath:     "/",
					DialFunc:         (&net.Dialer{}).DialContext,
				}, nil

			case cluster.ServiceName:
				return cluster.Mock(), nil
			case labelstore.ServiceName:
				return labelstore.New(nil, prometheus_client.DefaultRegisterer), nil
			case livedebugging.ServiceName:
				return livedebugging.NewLiveDebugging(), nil
			default:
				return nil, fmt.Errorf("service %q does not exist", name)
			}
		},
	}

	nilReceivers := []storage.Appendable{nil, nil}

	var args Arguments
	args.SetToDefault()
	args.ForwardTo = nilReceivers

	s, err := New(opts, args)
	require.NoError(t, err)

	// Forwarding samples to the nil receivers shouldn't fail.
	appender := s.appendable.Appender(t.Context())
	_, err = appender.Append(0, labels.FromStrings("foo", "bar"), 0, 0)
	require.NoError(t, err)

	err = appender.Commit()
	require.NoError(t, err)

	// Update the component with a mock receiver; it should be passed along to the Appendable.
	var receivedTs int64
	var receivedSamples labels.Labels
	ls := labelstore.New(nil, prometheus_client.DefaultRegisterer)
	fanout := prometheus.NewInterceptor(nil, ls, prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, t int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
		receivedTs = t
		receivedSamples = l
		return ref, nil
	}))
	require.NoError(t, err)
	args.ForwardTo = []storage.Appendable{fanout}
	err = s.Update(args)
	require.NoError(t, err)

	// Forwarding a sample to the mock receiver should succeed.
	appender = s.appendable.Appender(t.Context())
	timestamp := time.Now().Unix()
	sample := labels.FromStrings("foo", "bar")
	_, err = appender.Append(0, sample, timestamp, 42.0)
	require.NoError(t, err)

	err = appender.Commit()
	require.NoError(t, err)

	require.Equal(t, receivedTs, timestamp)
	require.Len(t, receivedSamples, 1)
	require.Equal(t, receivedSamples, sample)
}

// TestCustomDialer ensures that prometheus.scrape respects the custom dialer
// given to it.
func TestCustomDialer(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var (
		reg        = prometheus_client.NewRegistry()
		regHandler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

		scrapeTrigger = util.NewWaitTrigger()

		srv = &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				scrapeTrigger.Trigger()
				regHandler.ServeHTTP(w, r)
			}),
		}

		memLis = memconn.NewListener(util.TestLogger(t))
	)

	go srv.Serve(memLis)
	defer srv.Shutdown(ctx)

	var config = `
	targets         = [{ __address__ = "inmemory:80" }]
	forward_to      = []
	scrape_interval = "100ms"
	scrape_timeout  = "85ms"
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(config), &args)
	require.NoError(t, err)

	opts := component.Options{
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus_client.NewRegistry(),
		GetServiceData: func(name string) (interface{}, error) {
			switch name {
			case http_service.ServiceName:
				return http_service.Data{
					HTTPListenAddr:   "inmemory:80",
					MemoryListenAddr: "inmemory:80",
					BaseHTTPPath:     "/",
					DialFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
						return memLis.DialContext(ctx)
					},
				}, nil

			case cluster.ServiceName:
				return cluster.Mock(), nil
			case labelstore.ServiceName:
				return labelstore.New(nil, prometheus_client.DefaultRegisterer), nil
			case livedebugging.ServiceName:
				return livedebugging.NewLiveDebugging(), nil

			default:
				return nil, fmt.Errorf("service %q does not exist", name)
			}
		},
	}

	s, err := New(opts, args)
	require.NoError(t, err)
	go s.Run(ctx)

	// Wait for our scrape to be invoked.
	err = scrapeTrigger.Wait(1 * time.Minute)
	require.NoError(t, err, "custom dialer was not used")
}

func TestValidateScrapeConfig(t *testing.T) {
	var exampleAlloyConfig = `
	targets         = [{ "target1" = "target1" }]
	forward_to      = []
	scrape_interval = "10s"
	scrape_timeout  = "20s"
	job_name        = "local"
`
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "scrape_timeout (20s) greater than scrape_interval (10s) for scrape config with job name \"local\"")
}

func TestAlloyConfigDefaultsAndValidation(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectError   bool
		errorContains string
		assertions    func(t *testing.T, args Arguments)
	}{
		{
			name: "no validation scheme specified",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "utf8", args.MetricNameValidationScheme)
				require.Equal(t, "allow-utf-8", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "valid utf8 with allow-utf-8",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = "utf8"
				metric_name_escaping_scheme = "allow-utf-8"
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "utf8", args.MetricNameValidationScheme)
				require.Equal(t, "allow-utf-8", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "valid legacy with underscores",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = "legacy"
				metric_name_escaping_scheme = "underscores"
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "legacy", args.MetricNameValidationScheme)
				require.Equal(t, "underscores", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "invalid combination - allow-utf-8 with legacy validation",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = "legacy"
				metric_name_escaping_scheme = "allow-utf-8"
			`,
			expectError:   true,
			errorContains: "metric_name_escaping_scheme cannot be set to 'allow-utf-8' while metric_name_validation_scheme is not set to 'utf8'",
		},
		{
			name: "invalid validation scheme in config",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = "invalid"
			`,
			expectError:   true,
			errorContains: "invalid metric_name_validation_scheme",
		},
		{
			name: "invalid escaping scheme in config",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_escaping_scheme = "invalid"
			`,
			expectError:   true,
			errorContains: "invalid metric_name_escaping_scheme",
		},
		{
			name: "utf8 validation with dots escaping",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = "utf8"
				metric_name_escaping_scheme = "dots"
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "utf8", args.MetricNameValidationScheme)
				require.Equal(t, "dots", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "utf8 validation only - escaping scheme should default to allow-utf-8",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = "utf8"
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "utf8", args.MetricNameValidationScheme)
				require.Equal(t, "allow-utf-8", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "legacy validation only - escaping scheme should default to underscores",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = "legacy"
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "legacy", args.MetricNameValidationScheme)
				require.Equal(t, "underscores", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "escaping scheme only - validation scheme should default to utf8",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_escaping_scheme = "allow-utf-8"
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "utf8", args.MetricNameValidationScheme)
				require.Equal(t, "allow-utf-8", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "empty string validation scheme",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_validation_scheme = ""
			`,
			expectError:   true,
			errorContains: "invalid metric_name_validation_scheme",
		},
		{
			name: "empty string escaping scheme",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_escaping_scheme = ""
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "utf8", args.MetricNameValidationScheme)
				require.Equal(t, "allow-utf-8", args.MetricNameEscapingScheme)
			},
		},
		{
			name: "invalid scrape fallback protocol",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				scrape_fallback_protocol = "invalid"
			`,
			expectError:   true,
			errorContains: "invalid scrape_fallback_protocol",
		},
		{
			name: "valid scrape fallback protocol",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				scrape_fallback_protocol = "PrometheusProto"
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "PrometheusProto", args.ScrapeFallbackProtocol)
			},
		},
		{
			name: "native histogram defaults",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.False(t, args.ConvertClassicHistogramsToNHCB)
				require.True(t, args.EnableCompression)
				require.Equal(t, uint(0), args.NativeHistogramBucketLimit)
				require.Equal(t, 0.0, args.NativeHistogramMinBucketFactor)
			},
		},
		{
			name: "valid native histogram config",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				convert_classic_histograms_to_nhcb = true
				enable_compression = true
				native_histogram_bucket_limit = 160
				native_histogram_min_bucket_factor = 1.1
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.True(t, args.ConvertClassicHistogramsToNHCB)
				require.True(t, args.EnableCompression)
				require.Equal(t, uint(160), args.NativeHistogramBucketLimit)
				require.Equal(t, 1.1, args.NativeHistogramMinBucketFactor)
			},
		},
		{
			name: "valid native_histogram_min_bucket_factor - exactly 1.0",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				native_histogram_min_bucket_factor = 1.0
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, 1.0, args.NativeHistogramMinBucketFactor)
			},
		},
		{
			name: "valid native_histogram_min_bucket_factor - zero means no limit",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				native_histogram_min_bucket_factor = 0.0
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, 0.0, args.NativeHistogramMinBucketFactor)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tt.config), &args)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				// Validate that the config was parsed correctly
				err = args.Validate()
				require.NoError(t, err)

				// Check default values if validation function is provided
				if tt.assertions != nil {
					tt.assertions(t, args)
				}
			}
		})
	}
}
