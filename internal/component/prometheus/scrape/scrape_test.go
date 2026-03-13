package scrape

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
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

	opts := newRuntimeUpdateOpts(t, t.Name(), func(ctx context.Context, network, address string) (net.Conn, error) {
		return memLis.DialContext(ctx)
	})

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

	serverAddr := startMetricsServer(t, reg)

	// Set up test appender using the testappender utility
	appender := testappender.NewCollectingAppender()

	// Create appendable wrapper using testappender utility
	mockAppendable := testappender.ConstantAppendable{Inner: appender}

	// Set up component options
	opts := newRuntimeUpdateOpts(t, strings.ReplaceAll(t.Name(), " ", "_"))

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

// --- Helpers for TestRuntimeUpdate ---

// defaultFastScrapeArgs returns Arguments with defaults pointing at addr, a 50 ms
// scrape interval and a shared appendable. Validate() is NOT called so callers can
// modify fields before validating.
func defaultFastScrapeArgs(addr string, app storage.Appendable) Arguments {
	var args Arguments
	args.SetToDefault()
	args.Targets = []discovery.Target{
		discovery.NewTargetFromLabelSet(model.LabelSet{"__address__": model.LabelValue(addr)}),
	}
	args.ForwardTo = []storage.Appendable{app}
	args.ScrapeInterval = 50 * time.Millisecond
	args.ScrapeTimeout = 40 * time.Millisecond
	args.JobName = "test_job"
	return args
}

// hasSampleForMetric returns true when any sample in the map carries __name__ == name.
func hasSampleForMetric(samples map[string]*testappender.MetricSample, name string) bool {
	for _, s := range samples {
		if s.Labels.Get("__name__") == name {
			return true
		}
	}
	return false
}

// TestRuntimeUpdate verifies that config fields can (or gracefully cannot) be changed
// while the component is running, and that targets are always updated regardless.
//
// Each sub-test uses two real HTTP servers (A and B) that expose metrics with distinct
// names, so pre- and post-update assertions are unambiguous without any appender swapping.
func TestRuntimeUpdate(t *testing.T) {
	type testCase struct {
		name string
		// setupRegistryA/B build the metric registries that the two HTTP servers expose.
		setupRegistryA func(t *testing.T) *prometheus_client.Registry
		setupRegistryB func(t *testing.T) *prometheus_client.Registry
		// initialArgs / updatedArgs receive both server addresses and the shared appendable.
		initialArgs func(t *testing.T, addrA, addrB string, app storage.Appendable) Arguments
		updatedArgs func(t *testing.T, addrA, addrB string, app storage.Appendable) Arguments
		// preUpdateCheck is polled until it passes, then Update is called.
		preUpdateCheck func(ct *assert.CollectT, c testappender.CollectingAppender)
		// postUpdateCheck is polled after Update returns.
		// Because each test uses unique metric names for the pre- and post-update states,
		// the post-update metric can only appear after the update takes effect.
		postUpdateCheck   func(ct *assert.CollectT, c testappender.CollectingAppender)
		expectUpdateError bool
	}

	singleGaugeRegistry := func(name string) func(t *testing.T) *prometheus_client.Registry {
		return func(t *testing.T) *prometheus_client.Registry {
			reg := prometheus_client.NewRegistry()
			g := prometheus_client.NewGauge(prometheus_client.GaugeOpts{Name: name})
			g.Set(1)
			reg.MustRegister(g)
			return reg
		}
	}

	tests := []testCase{
		{
			// Changing targets must cause the new target to be scraped.
			// server_b_up can only appear once the target switches to server B.
			name:           "targets are updated",
			setupRegistryA: singleGaugeRegistry("server_a_up"),
			setupRegistryB: singleGaugeRegistry("server_b_up"),
			initialArgs: func(t *testing.T, addrA, _ string, app storage.Appendable) Arguments {
				args := defaultFastScrapeArgs(addrA, app)
				require.NoError(t, args.Validate())
				return args
			},
			updatedArgs: func(t *testing.T, _, addrB string, app storage.Appendable) Arguments {
				args := defaultFastScrapeArgs(addrB, app)
				require.NoError(t, args.Validate())
				return args
			},
			preUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_a_up"), "server_a_up should appear before update")
			},
			postUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				// TODO: Check that server A is no longer being scraped
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_b_up"), "server_b_up should appear after target update")
			},
		},
		{
			// Changing scrape_native_histograms is silently ignored (a warning is logged); the component
			// must keep running and must still apply the updated target list.
			// server_b_up can only appear once the target switches to server B.
			name:           "scrape_native_histograms change is ignored but component continues with updated targets",
			setupRegistryA: singleGaugeRegistry("server_a_up"),
			setupRegistryB: singleGaugeRegistry("server_b_up"),
			initialArgs: func(t *testing.T, addrA, _ string, app storage.Appendable) Arguments {
				args := defaultFastScrapeArgs(addrA, app)
				args.ScrapeNativeHistograms = false
				require.NoError(t, args.Validate())
				return args
			},
			updatedArgs: func(t *testing.T, _, addrB string, app storage.Appendable) Arguments {
				// Flip ScrapeNativeHistograms AND change the target. The ScrapeNativeHistograms change
				// must be ignored (warning logged) while the target change must still take effect.
				args := defaultFastScrapeArgs(addrB, app)
				args.ScrapeNativeHistograms = true
				require.NoError(t, args.Validate())
				return args
			},
			preUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_a_up"), "server_a_up should appear before update")
			},
			postUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_b_up"), "server_b_up should appear: targets must be updated even when scrape_native_histograms change is ignored")
			},
		},
		{
			// Changing extra_metrics is silently ignored (a warning is logged); the component
			// must keep running and must still apply the updated target list.
			// server_b_up can only appear once the target switches to server B.
			name:           "extra_metrics change is ignored but component continues with updated targets",
			setupRegistryA: singleGaugeRegistry("server_a_up"),
			setupRegistryB: singleGaugeRegistry("server_b_up"),
			initialArgs: func(t *testing.T, addrA, _ string, app storage.Appendable) Arguments {
				args := defaultFastScrapeArgs(addrA, app)
				args.ExtraMetrics = false
				require.NoError(t, args.Validate())
				return args
			},
			updatedArgs: func(t *testing.T, _, addrB string, app storage.Appendable) Arguments {
				// Flip ExtraMetrics AND change the target. The ExtraMetrics change must be
				// ignored (warning logged) while the target change must still take effect.
				args := defaultFastScrapeArgs(addrB, app)
				args.ExtraMetrics = true
				require.NoError(t, args.Validate())
				return args
			},
			preUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_a_up"), "server_a_up should appear before update")
			},
			postUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				// TODO: Also check the log for the warning
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_b_up"), "server_b_up should appear: targets must be updated even when extra_metrics change is ignored")
			},
		},
		{
			// When ApplyConfig fails, the deferred reloadTargets signal still fires and the
			// new targets stored in c.args are picked up by the Run loop.
			// A non-existent TLS CA file causes scrape.Manager.ApplyConfig to fail when it
			// tries to build the HTTP client, but the scrape manager keeps its previous
			// (TLS-free) config and can still reach plain-HTTP server B.
			// server_b_up can only appear once the target switches to server B.
			name:           "targets are updated even when ApplyConfig fails",
			setupRegistryA: singleGaugeRegistry("server_a_up"),
			setupRegistryB: singleGaugeRegistry("server_b_up"),
			initialArgs: func(t *testing.T, addrA, _ string, app storage.Appendable) Arguments {
				args := defaultFastScrapeArgs(addrA, app)
				require.NoError(t, args.Validate())
				return args
			},
			updatedArgs: func(t *testing.T, _, addrB string, app storage.Appendable) Arguments {
				args := defaultFastScrapeArgs(addrB, app)
				// A non-existent CA file passes Arguments.Validate() (file existence is not
				// checked there) but causes ApplyConfig to fail when building the TLS client.
				args.HTTPClientConfig.TLSConfig.CAFile = "/nonexistent/ca.pem"
				require.NoError(t, args.Validate())
				return args
			},
			preUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_a_up"), "server_a_up should appear before update")
			},
			postUpdateCheck: func(ct *assert.CollectT, c testappender.CollectingAppender) {
				assert.True(ct, hasSampleForMetric(c.CollectedSamples(), "server_b_up"), "server_b_up should appear even when ApplyConfig fails")
			},
			expectUpdateError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			addrA := startMetricsServer(t, tc.setupRegistryA(t))
			addrB := startMetricsServer(t, tc.setupRegistryB(t))

			appender := testappender.NewCollectingAppender()
			appendable := testappender.ConstantAppendable{Inner: appender}

			c, err := New(newRuntimeUpdateOpts(t, strings.ReplaceAll(tc.name, " ", "_")), tc.initialArgs(t, addrA, addrB, appendable))
			require.NoError(t, err)
			go c.Run(ctx)

			require.EventuallyWithT(t, func(ct *assert.CollectT) {
				tc.preUpdateCheck(ct, appender)
			}, 10*time.Second, 50*time.Millisecond, "pre-update check timed out")

			updateErr := c.Update(tc.updatedArgs(t, addrA, addrB, appendable))
			if tc.expectUpdateError {
				require.Error(t, updateErr)
			} else {
				require.NoError(t, updateErr)
			}

			require.EventuallyWithT(t, func(ct *assert.CollectT) {
				tc.postUpdateCheck(ct, appender)
			}, 10*time.Second, 50*time.Millisecond, "post-update check timed out")
		})
	}
}

// --- Helpers for all tests ---

// startMetricsServer starts a TCP HTTP server that serves reg at /metrics.
// The server is shut down automatically when the test ends.
func startMetricsServer(t *testing.T, reg *prometheus_client.Registry) string {
	t.Helper()
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true})
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/metrics" {
				handler.ServeHTTP(w, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return addr
}

// newRuntimeUpdateOpts returns component.Options suitable for tests.
// componentID is used as the component identifier and is included in every log line
// as component_path/component_id fields, mirroring production behaviour.
// An optional dialFunc overrides the default (*net.Dialer).DialContext, which is
// useful for tests that need an in-memory or otherwise custom transport.
func newRuntimeUpdateOpts(t *testing.T, componentID string, dialFunc ...func(context.Context, string, string) (net.Conn, error)) component.Options {
	t.Helper()
	df := (&net.Dialer{}).DialContext
	if len(dialFunc) > 0 && dialFunc[0] != nil {
		df = dialFunc[0]
	}
	baseLogger := util.TestAlloyLogger(t)
	return component.Options{
		ID:         componentID,
		Logger:     log.With(baseLogger, "component_path", "prometheus.scrape", "component_id", componentID),
		Registerer: prometheus_client.NewRegistry(),
		GetServiceData: func(name string) (any, error) {
			switch name {
			case http_service.ServiceName:
				return http_service.Data{
					HTTPListenAddr:   "localhost:0",
					MemoryListenAddr: "alloy.internal:0",
					BaseHTTPPath:     "/",
					DialFunc:         df,
				}, nil
			case cluster.ServiceName:
				return cluster.Mock(), nil
			case labelstore.ServiceName:
				return labelstore.New(nil, prometheus_client.NewRegistry()), nil
			case livedebugging.ServiceName:
				return livedebugging.NewLiveDebugging(), nil
			default:
				return nil, fmt.Errorf("service %q does not exist", name)
			}
		},
	}
}
