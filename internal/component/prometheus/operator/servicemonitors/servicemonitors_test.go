package servicemonitors

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/schema"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/component/prometheus/operator/common"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util/syncbuffer"
	"github.com/grafana/alloy/internal/util/testappender"
)

// TestServiceMonitorEndToEnd tests the complete flow:
// 1. Start prometheus.operator.servicemonitors component with fake k8s client
// 2. Add a ServiceMonitor that triggers scrape config generation
// 3. Verify metrics are received through forward_to
//
// This follows the mimir.alerts.kubernetes testing pattern where tests
// interact with internal components using fake dependencies.
func TestServiceMonitorEndToEnd(t *testing.T) {
	testCases := []struct {
		name           string
		honorMetadata  bool
		expectMetadata bool
	}{
		{
			name:           "honor_metadata_enabled",
			honorMetadata:  true,
			expectMetadata: true,
		},
		{
			name:           "honor_metadata_disabled",
			honorMetadata:  false,
			expectMetadata: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())

			// Start a test HTTP server that serves Prometheus metrics
			metricsServer, serverAddr := startTestMetricsServer(t)
			defer metricsServer.Close()

			// Create fake kubernetes client
			fakeK8s := fake.NewSimpleClientset()

			// Set up test appender to collect metrics
			appender := testappender.NewCollectingAppender()
			mockAppendable := testappender.ConstantAppendable{Inner: appender}

			// Create a synchronized log buffer to capture logs for detecting "informers started"
			logBuffer := &syncbuffer.Buffer{}

			// Create a logger that writes to both the buffer and stderr
			multiWriter := io.MultiWriter(logBuffer, os.Stderr)
			logger, err := logging.New(multiWriter, logging.Options{
				Level:  logging.LevelDebug,
				Format: logging.FormatLogfmt,
			})
			require.NoError(t, err)

			// Create component options
			opts := component.Options{
				ID:         "prometheus.operator.servicemonitors.test",
				Logger:     logger,
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
					default:
						return nil, fmt.Errorf("service %q does not exist", name)
					}
				},
			}

			// Create component arguments
			var args operator.Arguments
			args.SetToDefault()
			args.ForwardTo = []storage.Appendable{mockAppendable}
			args.Namespaces = []string{"monitoring"}
			args.Scrape.HonorMetadata = tc.honorMetadata

			// Create the component
			comp, err := common.New(opts, args, common.KindServiceMonitor)
			require.NoError(t, err)

			// Create a test factory that provides access to internal components
			testFactory := &common.TestCrdManagerFactory{
				K8sClient: fakeK8s,
				LogBuffer: logBuffer,
			}

			// Inject our test factory
			common.SetCrdManagerFactory(comp, testFactory)

			// Run the component in a goroutine
			var wg sync.WaitGroup
			var runErr error
			wg.Add(1)
			go func() {
				defer wg.Done()
				runErr = comp.Run(ctx)
			}()

			// Ensure we wait for the goroutine to exit before test completes
			defer func() {
				cancel() // Signal the component to stop
				wg.Wait()
				require.NoError(t, runErr)
			}()

			// Trigger an update to start the crdManager
			err = comp.Update(args)
			require.NoError(t, err)

			// Create a ServiceMonitor (similar to mimir tests adding AlertmanagerConfig to indexer)
			serviceMonitor := &promopv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "monitoring",
					Name:      "test-service-monitor",
				},
				Spec: promopv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
					Endpoints: []promopv1.Endpoint{
						{
							Port:          "metrics",
							Interval:      promopv1.Duration("1s"),
							ScrapeTimeout: promopv1.Duration("500ms"),
						},
					},
				},
			}

			// Trigger the add event. Retry until the manager is ready.
			require.Eventually(t, func() bool {
				return testFactory.TriggerServiceMonitorAdd(serviceMonitor)
			}, 10*time.Second, 100*time.Millisecond, "Timeout waiting for manager to be ready")

			// Verify scrape config was registered
			require.Eventually(t, func() bool {
				jobNames := testFactory.GetScrapeConfigJobNames()
				return len(jobNames) == 1
			}, 5*time.Second, 100*time.Millisecond, "Expected 1 scrape config to be registered")

			// Inject static targets (since k8s service discovery won't work without a real cluster)
			jobName := "serviceMonitor/monitoring/test-service-monitor/0"
			ready, err := testFactory.InjectStaticTargets(jobName, serverAddr)
			require.True(t, ready, "Manager should be ready after TriggerServiceMonitorAdd succeeded")
			require.NoError(t, err)

			// Wait for metrics to be scraped and forwarded, then verify
			require.EventuallyWithT(t, func(ct *assert.CollectT) {
				samples := appender.CollectedSamples()

				counterSample := findSampleByName(samples, "test_counter_total")
				assert.NotNil(ct, counterSample, "Expected to find test_counter_total metric")
				if counterSample != nil {
					assert.Equal(ct, float64(42), counterSample.Value)
				}

				gaugeSample := findSampleByName(samples, "test_gauge")
				assert.NotNil(ct, gaugeSample, "Expected to find test_gauge metric")
				if gaugeSample != nil {
					assert.Equal(ct, 3.14, gaugeSample.Value)
				}

				// Verify metadata handling based on honor_metadata setting
				allMetadata := appender.CollectedMetadata()
				counterMeta := findMetadataByName(allMetadata, "test_counter_total")
				gaugeMeta := findMetadataByName(allMetadata, "test_gauge")

				if tc.expectMetadata {
					assert.NotNil(ct, counterMeta, "Expected to find metadata for test_counter_total")
					if counterMeta != nil {
						assert.Equal(ct, "A test counter metric", counterMeta.Help)
					}

					assert.NotNil(ct, gaugeMeta, "Expected to find metadata for test_gauge")
					if gaugeMeta != nil {
						assert.Equal(ct, "A test gauge metric", gaugeMeta.Help)
					}
				} else {
					assert.Nil(ct, counterMeta, "Expected no metadata for test_counter_total when honor_metadata is disabled")
					assert.Nil(ct, gaugeMeta, "Expected no metadata for test_gauge when honor_metadata is disabled")
				}
			}, 30*time.Second, 500*time.Millisecond, "timed out waiting for metrics to be forwarded")
		})
	}
}

// startTestMetricsServer starts an HTTP server that serves Prometheus metrics.
// The server is automatically closed when the test completes.
func startTestMetricsServer(t *testing.T) (*http.Server, string) {
	t.Helper()

	// Create a new registry for test metrics
	reg := prometheus_client.NewRegistry()

	// Create test metrics
	counter := prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "test_counter_total",
		Help: "A test counter metric",
	})
	gauge := prometheus_client.NewGauge(prometheus_client.GaugeOpts{
		Name: "test_gauge",
		Help: "A test gauge metric",
	})

	reg.MustRegister(counter, gauge)

	// Set initial values
	counter.Add(42)
	gauge.Set(3.14)

	// Create handler
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	// Start server on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &http.Server{
		Handler: handler,
	}

	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		// Intentionally not logging errors here to avoid race with test completion.
		// We expect http.ErrServerClosed when the server is shut down.
		_ = server.Serve(listener)
	}()

	// Use t.Cleanup to ensure the server goroutine is properly waited for
	// before the test completes, avoiding races with t.Logf or other test state.
	t.Cleanup(func() {
		server.Close()
		serverWg.Wait()
	})

	// Wait for server to be ready
	time.Sleep(50 * time.Millisecond)

	return server, listener.Addr().String()
}

// findSampleByName finds a sample by metric name from the collected samples
func findSampleByName(samples map[string]*testappender.MetricSample, metricName string) *testappender.MetricSample {
	for _, sample := range samples {
		if schema.NewMetadataFromLabels(sample.Labels).Name == metricName {
			return sample
		}
	}
	return nil
}

// findMetadataByName finds metadata by metric name from the collected metadata
func findMetadataByName(allMetadata map[string]metadata.Metadata, metricName string) *metadata.Metadata {
	// The key is labels.String() format: {__name__="metric_name", ...}
	searchStr := fmt.Sprintf(`__name__="%s"`, metricName)
	for labelStr, meta := range allMetadata {
		if strings.Contains(labelStr, searchStr) {
			return &meta
		}
	}
	return nil
}
