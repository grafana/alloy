package scrape

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grafana/ckit/memconn"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/schema"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	component_config "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
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
		GetServiceData: func(name string) (any, error) {
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
			name: "defaults",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "legacy", args.MetricNameValidationScheme)
				require.Equal(t, "underscores", args.MetricNameEscapingScheme)
				require.Equal(t, []string{
					"OpenMetricsText1.0.0",
					"OpenMetricsText0.0.1",
					"PrometheusText1.0.0",
					"PrometheusText0.0.4",
				}, args.ScrapeProtocols)
				require.Equal(t, "PrometheusText0.0.4", args.ScrapeFallbackProtocol)
				require.Equal(t, false, args.ScrapeNativeHistograms)
				require.Equal(t, false, args.ConvertClassicHistogramsToNHCB)
				require.Equal(t, true, args.EnableCompression)
				require.Equal(t, uint(0), args.NativeHistogramBucketLimit)
				require.Equal(t, 0.0, args.NativeHistogramMinBucketFactor)
			},
		},
		{
			name: "native histogram defaults",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				scrape_native_histograms = true
			`,
			expectError: false,
			assertions: func(t *testing.T, args Arguments) {
				require.Equal(t, "legacy", args.MetricNameValidationScheme)
				require.Equal(t, "underscores", args.MetricNameEscapingScheme)
				require.Equal(t, []string{
					"PrometheusProto",
					"OpenMetricsText1.0.0",
					"OpenMetricsText0.0.1",
					"PrometheusText1.0.0",
					"PrometheusText0.0.4",
				}, args.ScrapeProtocols)
				require.Equal(t, "PrometheusText0.0.4", args.ScrapeFallbackProtocol)
				require.Equal(t, true, args.ScrapeNativeHistograms)
				require.Equal(t, false, args.ConvertClassicHistogramsToNHCB)
				require.Equal(t, true, args.EnableCompression)
				require.Equal(t, uint(0), args.NativeHistogramBucketLimit)
				require.Equal(t, 0.0, args.NativeHistogramMinBucketFactor)
			},
		},
		{
			name: "native histogram missing PrometheusProto",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				scrape_native_histograms = true
				scrape_protocols = ["OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText1.0.0"]
			`,
			expectError:   true,
			errorContains: `scrape_native_histograms is set to true, but PrometheusProto is not in scrape_protocols`,
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
			name: "escaping scheme only set to utf-8",
			config: `
				targets = [{ "target1" = "target1" }]
				forward_to = []
				metric_name_escaping_scheme = "allow-utf-8"
			`,
			expectError:   true,
			errorContains: "metric_name_escaping_scheme cannot be set to 'allow-utf-8' while metric_name_validation_scheme is not set to 'utf8'",
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
				require.Equal(t, "legacy", args.MetricNameValidationScheme)
				require.Equal(t, "underscores", args.MetricNameEscapingScheme)
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

// setupTestCounter creates and initializes a test counter metric
func setupTestCounter() prometheus_client.Counter {
	counter := prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "test_counter_total",
		Help: "A test counter metric",
	})
	counter.Add(42.5)
	return counter
}

// setupTestGauge creates and initializes a test gauge metric
func setupTestGauge() prometheus_client.Gauge {
	gauge := prometheus_client.NewGauge(prometheus_client.GaugeOpts{
		Name: "test_gauge",
		Help: "A test gauge metric",
	})
	gauge.Set(123.45)
	return gauge
}

// setupTestHistogram creates and initializes a test classic histogram metric
func setupTestHistogram() prometheus_client.Histogram {
	histogram := prometheus_client.NewHistogram(prometheus_client.HistogramOpts{
		Name:    "test_histogram",
		Help:    "A test histogram metric",
		Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
	})
	// Add observations to match expected values
	histogram.Observe(0.3) // Falls in 0.5 bucket
	histogram.Observe(0.7) // Falls in 1.0 bucket
	histogram.Observe(1.2) // Falls in 2.5 bucket
	histogram.Observe(4.5) // Falls in 5.0 bucket
	return histogram
}

// setupTestNativeHistogram creates and initializes a test native histogram metric
func setupTestNativeHistogram() prometheus_client.Histogram {
	nativeHistogram := prometheus_client.NewHistogram(prometheus_client.HistogramOpts{
		Name:                            "test_native_histogram",
		Help:                            "A test native histogram metric",
		NativeHistogramBucketFactor:     1.1,
		NativeHistogramMaxBucketNumber:  100,
		NativeHistogramMinResetDuration: 1 * time.Hour,
	})
	// Add observations to match expected values (total sum: 1.5 + 2.8 + 3.5 = 7.8)
	nativeHistogram.Observe(1.5)
	nativeHistogram.Observe(2.8)
	nativeHistogram.Observe(3.5)
	return nativeHistogram
}

// setupTestMixedHistogram creates and initializes a test histogram with both classic and native buckets
func setupTestMixedHistogram() prometheus_client.Histogram {
	mixedHistogram := prometheus_client.NewHistogram(prometheus_client.HistogramOpts{
		Name:    "test_mixed_histogram",
		Help:    "A test histogram metric with both classic and native buckets",
		Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0}, // Classic buckets
		// Native histogram configuration
		NativeHistogramBucketFactor:     1.1,
		NativeHistogramMaxBucketNumber:  100,
		NativeHistogramMinResetDuration: 1 * time.Hour,
	})
	// Add observations to create both classic and native data
	mixedHistogram.Observe(0.3) // Falls in 0.5 bucket
	mixedHistogram.Observe(0.7) // Falls in 1.0 bucket
	mixedHistogram.Observe(1.2) // Falls in 2.5 bucket
	mixedHistogram.Observe(4.5) // Falls in 5.0 bucket
	mixedHistogram.Observe(8.1) // Falls in 10.0 bucket
	return mixedHistogram
}

// setupTestSummary creates and initializes a test summary metric
func setupTestSummary() prometheus_client.Summary {
	summary := prometheus_client.NewSummary(prometheus_client.SummaryOpts{
		Name: "test_summary",
		Help: "A test summary metric",
		Objectives: map[float64]float64{
			0.5:  0.05,
			0.9:  0.01,
			0.99: 0.001,
		},
	})
	// Add observations to get expected count and sum (total sum: 0.5 + 0.9 + 1.5 + 0.0 = 2.9)
	summary.Observe(0.5)
	summary.Observe(0.9)
	summary.Observe(1.5)
	summary.Observe(0.0)
	return summary
}

// setupTestMetrics creates a registry with all test metrics
func setupTestMetrics() *prometheus_client.Registry {
	reg := prometheus_client.NewRegistry()

	counter := setupTestCounter()
	gauge := setupTestGauge()
	histogram := setupTestHistogram()
	nativeHistogram := setupTestNativeHistogram()
	mixedHistogram := setupTestMixedHistogram()
	summary := setupTestSummary()

	reg.MustRegister(counter, gauge, histogram, nativeHistogram, mixedHistogram, summary)
	return reg
}

func TestScrapingAllMetricTypes(t *testing.T) {
	// Test both with and without type and unit labels
	for _, enableTypeAndUnitLabels := range []bool{false, true} {
		testName := fmt.Sprintf("EnableTypeAndUnitLabels=%t", enableTypeAndUnitLabels)
		t.Run(testName, func(t *testing.T) {
			testScrapingAllMetricTypes(t, enableTypeAndUnitLabels)
		})
	}
}

func testScrapingAllMetricTypes(t *testing.T, enableTypeAndUnitLabels bool) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	expectedSamples := []struct {
		name  string
		value float64
	}{
		{name: "test_counter_total", value: 42.5},
		{name: "test_gauge", value: 123.45},
		// Histogram samples
		{name: "test_histogram_count", value: 4.0},
		{name: "test_histogram_sum", value: 6.7}, // 0.3 + 0.7 + 1.2 + 4.5
		// Mixed histogram samples (classic samples expected for mixed histograms with both classic and native buckets)
		{name: "test_mixed_histogram_count", value: 5.0},
		{name: "test_mixed_histogram_sum", value: 14.8}, // 0.3 + 0.7 + 1.2 + 4.5 + 8.1
		// Summary samples
		{name: "test_summary_count", value: 4.0},
		{name: "test_summary_sum", value: 2.9}, // 0.5 + 0.9 + 1.5 + 0.0
	}

	expectedMetadata := []struct {
		name         string
		expectedType model.MetricType
		expectedHelp string
	}{
		{
			name:         "test_counter_total",
			expectedType: model.MetricTypeCounter,
			expectedHelp: "A test counter metric",
		},
		{
			name:         "test_gauge",
			expectedType: model.MetricTypeGauge,
			expectedHelp: "A test gauge metric",
		},
		{
			name:         "test_histogram_bucket",
			expectedType: model.MetricTypeHistogram,
			expectedHelp: "A test histogram metric",
		},
		{
			name:         "test_native_histogram",
			expectedType: model.MetricTypeHistogram,
			expectedHelp: "A test native histogram metric",
		},
		{
			name:         "test_mixed_histogram",
			expectedType: model.MetricTypeHistogram,
			expectedHelp: "A test histogram metric with both classic and native buckets",
		},
		{
			name:         "test_summary",
			expectedType: model.MetricTypeSummary,
			expectedHelp: "A test summary metric",
		},
	}

	expectedHistograms := []struct {
		name          string
		expectedCount uint64
		expectedSum   float64
	}{
		{
			name:          "test_native_histogram",
			expectedCount: 3,
			expectedSum:   7.8,
		},
		{
			name:          "test_mixed_histogram",
			expectedCount: 5,
			expectedSum:   14.8,
		},
	}

	// Create a Prometheus registry and metrics for protobuf format
	reg := setupTestMetrics()

	// Create HTTP server that serves metrics in protobuf format
	server := &http.Server{
		Addr: "127.0.0.1:0", // Let the OS choose a free port
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/metrics" {
				// Create a promhttp handler that prefers protobuf format
				handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{
					EnableOpenMetrics: true,
				})
				handler.ServeHTTP(w, r)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}),
	}

	listener, err := net.Listen("tcp", server.Addr)
	require.NoError(t, err)
	serverAddr := listener.Addr().String()

	go func() {
		server.Serve(listener)
	}()
	defer server.Shutdown(ctx)

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Test that the server is working by making a direct request
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", serverAddr))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Set up test appender using the testappender utility
	appender := testappender.NewCollectingAppender()

	// Create appendable wrapper using testappender utility
	mockAppendable := testappender.ConstantAppendable{Inner: appender}

	// Set up component options
	opts := component.Options{
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus_client.NewRegistry(),
		GetServiceData: func(name string) (any, error) {
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

	// Configure scrape arguments
	var args Arguments
	args.SetToDefault()
	args.HonorMetadata = true
	args.EnableTypeAndUnitLabels = enableTypeAndUnitLabels
	args.Targets = []discovery.Target{
		discovery.NewTargetFromLabelSet(model.LabelSet{"__address__": model.LabelValue(serverAddr)}),
	}
	args.ForwardTo = []storage.Appendable{mockAppendable}
	args.ScrapeInterval = 50 * time.Millisecond // Frequent scraping for test
	args.ScrapeTimeout = 25 * time.Millisecond
	args.JobName = "test_job"
	args.MetricsPath = "/metrics"
	args.ScrapeNativeHistograms = true  // Enable native histogram scraping
	args.ScrapeClassicHistograms = true // Enable classic histogram scraping
	args.ScrapeProtocols = []string{
		"PrometheusProto",
		"OpenMetricsText1.0.0",
		"OpenMetricsText0.0.1",
		"PrometheusText0.0.4",
	}

	// Validate arguments to set default values for validation and escaping schemes
	require.NoError(t, args.Validate())

	// Create and start the scrape component
	scrapeComponent, err := New(opts, args)
	require.NoError(t, err)

	go scrapeComponent.Run(ctx)

	// Wait for scraping to occur and commit data
	require.EventuallyWithT(t, func(collectT *assert.CollectT) {
		// Check if appender has collected samples, metadata, and histograms
		actualSamples := appender.CollectedSamples()
		actualMetadata := appender.CollectedMetadata()
		actualHistograms := appender.CollectedHistograms()

		// Verify we have captured samples
		require.Greater(collectT, len(actualSamples), 0, "Should have captured some samples")

		// Verify we have captured metadata
		require.Greater(collectT, len(actualMetadata), 0, "Should have captured some metadata")

		// Verify we have captured native histograms (since we enabled native histogram scraping)
		require.Greater(collectT, len(actualHistograms), 0, "Should have captured some native histograms")
	}, 10*time.Second, 100*time.Millisecond, "Should have captured samples, metadata, and histograms")

	// Get the collected samples and metadata
	actualSamples := appender.CollectedSamples()
	actualMetadata := appender.CollectedMetadata()
	actualHistograms := appender.CollectedHistograms()

	// First loop: Validate samples
	for _, expected := range expectedSamples {
		found := false
		for _, sample := range actualSamples {
			if sample.Labels.Get("__name__") == expected.name {
				require.Equal(t, expected.value, sample.Value, "Value should match for sample %s", expected.name)

				metadata := schema.NewMetadataFromLabels(sample.Labels)
				if enableTypeAndUnitLabels {
					require.NotEqual(t, model.MetricTypeUnknown, metadata.Type)
					// Unit is not current exposed to configure in client_golang https://github.com/prometheus/client_golang/pull/1392
					// require.NotEmpty(t, metadata.Unit)
				} else {
					require.Equal(t, model.MetricTypeUnknown, metadata.Type)
					require.Empty(t, metadata.Unit)
				}

				found = true
				break
			}
		}
		require.True(t, found, "Should have found expected sample: %s", expected.name)
	}

	// Second loop: Validate metadata
	for _, expected := range expectedMetadata {
		found := false
		for labelString, meta := range actualMetadata {
			// Check if this metadata entry is for our expected metric by looking for the metric name in the label string
			if strings.Contains(labelString, fmt.Sprintf(`__name__="%s"`, expected.name)) {
				require.Equal(t, expected.expectedType, meta.Type, "Metadata type should match for %s", expected.name)
				require.Equal(t, expected.expectedHelp, meta.Help, "Metadata help should match for %s", expected.name)
				found = true
				break
			}
		}
		require.True(t, found, "Should have found expected metadata: %s", expected.name)
	}

	// Third loop: Validate histograms
	for _, expected := range expectedHistograms {
		found := false
		for _, histogram := range actualHistograms {
			if histogram.Labels.Get("__name__") == expected.name {
				if histogram.Histogram != nil {
					require.Equal(t, expected.expectedCount, histogram.Histogram.Count, "Histogram count should match for %s", expected.name)
					require.Equal(t, expected.expectedSum, histogram.Histogram.Sum, "Histogram sum should match for %s", expected.name)
				} else if histogram.FloatHistogram != nil {
					require.Equal(t, float64(expected.expectedCount), histogram.FloatHistogram.Count, "Float histogram count should match for %s", expected.name)
					require.Equal(t, expected.expectedSum, histogram.FloatHistogram.Sum, "Float histogram sum should match for %s", expected.name)
				}

				metadata := schema.NewMetadataFromLabels(histogram.Labels)
				if enableTypeAndUnitLabels {
					require.NotEqual(t, model.MetricTypeUnknown, metadata.Type)
					// Unit is not current exposed to configure in client_golang https://github.com/prometheus/client_golang/pull/1392
					// require.NotEmpty(t, metadata.Unit)
				} else {
					require.Equal(t, model.MetricTypeUnknown, metadata.Type)
					require.Empty(t, metadata.Unit)
				}

				found = true
				break
			}
		}
		require.True(t, found, "Should have found expected histogram: %s", expected.name)
	}

	// Verify job label was added correctly
	for _, sample := range actualSamples {
		job := sample.Labels.Get("job")
		require.Equal(t, "test_job", job, "Job label should be added to all metrics")
	}

	t.Logf("Successfully scraped %d samples with %d metadata entries and %d histograms", len(actualSamples), len(actualMetadata), len(actualHistograms))
}
