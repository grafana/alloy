package secretfilter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
	"github.com/jaswdr/faker/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

// fakeSecret represents a fake secret to be used in the tests
type fakeSecret struct {
	name  string
	value string
}

// testLog represents a log entry to be used in the tests
type testLog struct {
	log     string
	secrets []fakeSecret // List of fake secrets it contains for easy redaction check
}

// List of fake secrets to use for testing
// They are constructed so that they will match the regexes in the gitleaks configs
// Note that some string literals are concatenated to avoid being flagged as secrets
var fakeSecrets = map[string]fakeSecret{
	"grafana-api-key": {
		name:  "grafana-api-key",
		value: "eyJr" + "IjoiT0x6NWJuNDRuZkI1NHJ6dEJrR0g3aDVuRnY0NWJuNDRuZkI1NHJ6dEJrR0g3aDVuRnY0NWJuNDRuZkI1NHJ6dEJrR0g3aDVuRnY0NWJuNDRuZkI1NHJ6dEJrR0g3aDVu",
	},
	"gcp-api-key": {
		name:  "gcp-api-key",
		value: "AIza" + "SyDaGmWKa4JsXZ-HjGw7ISLn_3namBGewQe",
	},
	"stripe-key": {
		name:  "stripe-access-token",
		value: "sk_live_" + "51HFxYz2eZvKYlo2C9kKM5nE6qO4yKn8N3bP7hXxYz2eZvKYlo2C",
	},
	"npm-token": {
		name:  "npm-access-token",
		value: "npm_" + "1A2b3C4d5E6f7G8h9I0jK1lM2nO3pQ4rS5tU",
	},
	"generic-api-key": {
		name:  "generic-api-key",
		value: "token:" + "Aa1Bb2Cc3Dd4Ee5Ff6Gg7Hh8Ii9Jj0Kk",
	},
}

// List of fake log entries to use for testing
var testLogs = map[string]testLog{
	"no_secret": {
		log: `{
			"message": "This is a simple log message"
		}`,
		secrets: []fakeSecret{},
	},
	"grafana_api_key": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["grafana-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["grafana-api-key"]},
	},
	"gcp_api_key": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["gcp-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["gcp-api-key"]},
	},
	"stripe_key": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["stripe-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["stripe-key"]},
	},
	"npm_token": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["npm-token"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["npm-token"]},
	},
	"generic_api_key": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["generic-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["generic-api-key"]},
	},
	"multiple_secrets": {
		log: `{
			"message": "This is a simple log message with a secret value ` + fakeSecrets["grafana-api-key"].value + ` and another secret value ` + fakeSecrets["gcp-api-key"].value + ` !
		}`,
		secrets: []fakeSecret{fakeSecrets["grafana-api-key"], fakeSecrets["gcp-api-key"]},
	},
}

// Alloy configurations for testing
var testConfigs = map[string]string{
	"default": `
		forward_to = []
	`,
	"with_origin": `
		forward_to = []
		origin_label = "job"
	`,
}

// Test cases for the secret filter
var tt = []struct {
	name         string
	config       string
	inputLog     string
	shouldRedact bool // Whether we expect any redaction to occur
}{
	{
		"no_secret",
		testConfigs["default"],
		testLogs["no_secret"].log,
		false,
	},
	{
		"grafana_api_key",
		testConfigs["default"],
		testLogs["grafana_api_key"].log,
		true,
	},
	{
		"gcp_api_key",
		testConfigs["default"],
		testLogs["gcp_api_key"].log,
		true,
	},
	{
		"stripe_key",
		testConfigs["default"],
		testLogs["stripe_key"].log,
		true,
	},
	{
		"npm_token",
		testConfigs["default"],
		testLogs["npm_token"].log,
		true,
	},
	{
		"generic_api_key",
		testConfigs["default"],
		testLogs["generic_api_key"].log,
		true,
	},
	{
		"multiple_secrets",
		testConfigs["default"],
		testLogs["multiple_secrets"].log,
		true,
	},
}

func TestSecretFiltering(t *testing.T) {
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, tc.config, tc.inputLog, tc.shouldRedact)
		})
	}
}

func runTest(t *testing.T, config string, inputLog string, shouldRedact bool) {
	ch1 := loki.NewLogsReceiver()
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(config), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}

	// Making sure we're not testing with an empty log line by mistake
	require.NotEmpty(t, inputLog)

	// Create component
	tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.secretfilter")
	require.NoError(t, err)

	// Run it
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err1 := tc.Run(ctx, args)
		require.NoError(t, err1)
		wg.Done()
	}()
	require.NoError(t, tc.WaitExports(time.Second))

	// Get the input channel
	input := tc.Exports().(Exports).Receiver

	// Send the log to the secret filter
	entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: inputLog}}
	input.Chan() <- entry
	tc.WaitRunning(time.Second * 10)

	// Check the output
	select {
	case logEntry := <-ch1.Chan():
		require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
		if shouldRedact {
			// Verify that the output is different from the input (something was redacted)
			require.NotEqual(t, inputLog, logEntry.Entry.Line, "Expected log to be redacted but it was not")
		} else {
			// Verify that the output is the same as the input (nothing was redacted)
			require.Equal(t, inputLog, logEntry.Entry.Line, "Expected log to remain unchanged but it was modified")
		}
	case <-time.After(5 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	// Stop the component
	cancel()
	wg.Wait()
}

func BenchmarkAllTypesNoSecret(b *testing.B) {
	// Run benchmarks with no secrets in the logs, with all regexes enabled
	runBenchmarks(b, testConfigs["default"], 0, "")
}

func BenchmarkAllTypesWithSecret(b *testing.B) {
	// Run benchmarks with secrets in the logs (20% of log entries), with all regexes enabled
	runBenchmarks(b, testConfigs["default"], 20, "gcp_api_key")
}

func BenchmarkAllTypesWithLotsOfSecrets(b *testing.B) {
	// Run benchmarks with secrets in the logs (80% of log entries), with all regexes enabled
	runBenchmarks(b, testConfigs["default"], 80, "gcp_api_key")
}

func runBenchmarks(b *testing.B, config string, percentageSecrets int, secretName string) {
	ch1 := loki.NewLogsReceiver()
	var args Arguments
	require.NoError(b, syntax.Unmarshal([]byte(config), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}

	opts := component.Options{
		Logger:         &noopLogger{}, // Disable logging so that it keeps a clean benchmark output
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}

	// Create component
	c, err := New(opts, args)
	require.NoError(b, err)

	// Generate fake log entries with a fixed seed so that it's reproducible
	fake := faker.NewWithSeed(rand.NewPCG(uint64(2014), uint64(2014)))
	nbLogs := 100
	benchInputs := make([]string, nbLogs)
	for i := range benchInputs {
		beginningStr := fake.Lorem().Paragraph(2)
		middleStr := fake.Lorem().Sentence(10)
		endingStr := fake.Lorem().Paragraph(2)

		// Add fake secrets in some log entries
		if secretName != "" && i < nbLogs*percentageSecrets/100 {
			middleStr = testLogs[secretName].log
		}

		benchInputs[i] = beginningStr + middleStr + endingStr
	}

	// Run benchmarks
	for i := 0; i < b.N; i++ {
		for _, input := range benchInputs {
			entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: input}}
			c.processEntry(entry)
		}
	}
}

var sampleFuzzLogLines = []string{
	`key=value1,value2 log=fmt test=1 secret=password`,
	`{"key":["value1","value2"],"log":"fmt","test":1,"secret":"password"}`,
	`1970-01-01 00:00:00 pattern value1,value2 1 secret`,
}

func FuzzProcessEntry(f *testing.F) {
	for _, line := range sampleFuzzLogLines {
		f.Add(line)
	}
	for _, testLog := range testLogs {
		f.Add(testLog.log)
	}

	opts := component.Options{
		Logger:         util.TestLogger(f),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}
	ch1 := loki.NewLogsReceiver()

	// Create component with default config
	var args Arguments
	require.NoError(f, syntax.Unmarshal([]byte(testConfigs["default"]), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}
	c, err := New(opts, args)
	require.NoError(f, err)

	f.Fuzz(func(t *testing.T, log string) {
		entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: log}}
		c.processEntry(entry)
	})
}

func getServiceData(name string) (any, error) {
	switch name {
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}

type noopLogger struct{}

func (d *noopLogger) Log(_ ...any) error {
	return nil
}

// TestMetrics verifies that the metrics for the secretfilter component are
// correctly registered and incremented.
func TestMetrics(t *testing.T) {
	tests := []struct {
		name                   string
		inputLog               string
		expectedRedactedTotal  int
		expectedRedactedByRule map[string]int
	}{
		{
			name:                  "No secrets",
			inputLog:              testLogs["no_secret"].log,
			expectedRedactedTotal: 0,
		},
		{
			name:                  "Single Grafana API key secret",
			inputLog:              testLogs["grafana_api_key"].log,
			expectedRedactedTotal: 1,
			expectedRedactedByRule: map[string]int{
				"grafana-api-key": 1,
			},
		},
		{
			name:                  "Multiple secrets",
			inputLog:              testLogs["multiple_secrets"].log,
			expectedRedactedTotal: 2,
			expectedRedactedByRule: map[string]int{
				"grafana-api-key": 1,
				"gcp-api-key":     1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new registry to collect metrics
			registry := prometheus.NewRegistry()

			// Initialize Arguments
			args := Arguments{
				ForwardTo:   []loki.LogsReceiver{loki.NewLogsReceiver()},
				OriginLabel: "job",
			}

			// Create options with the test registry
			opts := component.Options{
				Logger:         util.TestLogger(t),
				OnStateChange:  func(e component.Exports) {},
				GetServiceData: getServiceData,
				Registerer:     registry,
			}

			// Create component
			c, err := New(opts, args)
			require.NoError(t, err)

			// Create a test entry with labels
			labels := model.LabelSet{
				"job":      "test-job",
				"instance": "test-instance",
			}
			entry := loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      tc.inputLog,
				},
			}

			// Process the entry
			c.processEntry(entry)

			// Verify the metrics

			// Check secretsRedactedTotal
			if tc.expectedRedactedTotal > 0 {
				require.Equal(t, float64(tc.expectedRedactedTotal),
					testutil.ToFloat64(c.metrics.secretsRedactedTotal),
					"secretsRedactedTotal metric value is incorrect")
			}

			// Check secretsRedactedByRule - combine all metrics in a single string
			if len(tc.expectedRedactedByRule) > 0 {
				var metricStrings strings.Builder
				metricStrings.WriteString("# HELP loki_secretfilter_secrets_redacted_by_rule_total Number of secrets redacted, partitioned by rule name.\n")
				metricStrings.WriteString("# TYPE loki_secretfilter_secrets_redacted_by_rule_total counter\n")

				// Add each rule metric
				for ruleName, expectedCount := range tc.expectedRedactedByRule {
					metric := fmt.Sprintf(`loki_secretfilter_secrets_redacted_by_rule_total{rule="%s"} %d`,
						ruleName, expectedCount)
					metricStrings.WriteString(metric + "\n")
				}

				// Compare all the metrics at once
				require.NoError(t,
					testutil.GatherAndCompare(registry, strings.NewReader(metricStrings.String()),
						"loki_secretfilter_secrets_redacted_by_rule_total"))
			}

			// Check secretsRedactedByOrigin when redactions occurred
			if tc.expectedRedactedTotal > 0 {
				// Build expected origin label metric
				var metricStrings strings.Builder
				metricStrings.WriteString("# HELP loki_secretfilter_secrets_redacted_by_origin Number of secrets redacted, partitioned by origin label value.\n")
				metricStrings.WriteString("# TYPE loki_secretfilter_secrets_redacted_by_origin counter\n")

				// Add origin label metric
				if jobValue, exists := labels[model.LabelName("job")]; exists {
					metric := fmt.Sprintf(`loki_secretfilter_secrets_redacted_by_origin{origin="%s"} %d`,
						jobValue, tc.expectedRedactedTotal)
					metricStrings.WriteString(metric + "\n")
				}

				// Compare the metrics
				require.NoError(t,
					testutil.GatherAndCompare(registry, strings.NewReader(metricStrings.String()),
						"loki_secretfilter_secrets_redacted_by_origin"))
			}

			// Check processingDuration metric
			// We don't validate the exact value since it will vary, but we verify it exists and has the right structure
			count, err := testutil.GatherAndCount(registry, "loki_secretfilter_processing_duration_seconds")
			require.NoError(t, err)
			require.Equal(t, count, 1, "processingDuration metric should be registered")

			// We only check that the metric exists with the right type, not the actual values
			require.NoError(t, err, "processingDuration metric should be properly registered")

			// Additionally check that the metric has count > 0 (indicating it was observed at least once)
			metricFamilies, err := registry.Gather()
			require.NoError(t, err)

			var foundMetric bool
			for _, mf := range metricFamilies {
				if mf.GetName() == "loki_secretfilter_processing_duration_seconds" {
					foundMetric = true
					for _, m := range mf.GetMetric() {
						summary := m.GetSummary()
						require.NotNil(t, summary, "should have a summary metric")
						require.Greater(t, summary.GetSampleCount(), uint64(0), "summary should have samples")
					}
				}
			}
			require.True(t, foundMetric, "processingDuration metric should be gathered")
		})
	}
}

// Test to verify that the component registers its metrics with the registry
func TestMetricsRegistration(t *testing.T) {
	registry := prometheus.NewRegistry()

	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
		Registerer:     registry,
		ID:             "test_secretfilter",
	}

	// Create component with empty arguments
	args := Arguments{
		ForwardTo:   []loki.LogsReceiver{loki.NewLogsReceiver()},
		OriginLabel: "job",
	}

	c, err := New(opts, args)
	require.NoError(t, err)

	// Increment all metrics to ensure they will be gathered
	c.metrics.secretsRedactedTotal.Inc()
	c.metrics.secretsRedactedByRule.WithLabelValues("test_rule").Inc()
	c.metrics.secretsRedactedByOrigin.WithLabelValues("test_value").Inc()
	c.metrics.processingDuration.Observe(0.123)

	// Check that the metrics are registered
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	// Create a map of expected metrics
	expectedMetrics := map[string]bool{
		"loki_secretfilter_secrets_redacted_total":         false,
		"loki_secretfilter_secrets_redacted_by_rule_total": false,
		"loki_secretfilter_secrets_redacted_by_origin":     false,
		"loki_secretfilter_processing_duration_seconds":    false,
	}

	// Check each metric family
	for _, metricFamily := range metricFamilies {
		name := metricFamily.GetName()
		if _, exists := expectedMetrics[name]; exists {
			expectedMetrics[name] = true
		}
	}

	// Verify all expected metrics were found
	for metric, found := range expectedMetrics {
		require.True(t, found, "Expected metric %s to be registered", metric)
	}
}

// Test metrics for secrets across multiple log lines
func TestMetricsMultipleEntries(t *testing.T) {
	registry := prometheus.NewRegistry()

	args := Arguments{
		ForwardTo:   []loki.LogsReceiver{loki.NewLogsReceiver()},
		OriginLabel: "job",
	}

	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
		Registerer:     registry,
	}

	c, err := New(opts, args)
	require.NoError(t, err)

	// Process multiple entries with secrets
	entries := []loki.Entry{
		{
			Labels: model.LabelSet{"job": "test1"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["grafana_api_key"].log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test2"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["gcp_api_key"].log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test3"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["no_secret"].log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test4"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testLogs["grafana_api_key"].log,
			},
		},
	}

	for _, entry := range entries {
		c.processEntry(entry)
	}

	// Verify the metrics
	// We should have 3 redacted secrets (2 grafana-api-key and 1 gcp-api-key)
	require.Equal(t, float64(3), testutil.ToFloat64(c.metrics.secretsRedactedTotal),
		"secretsRedactedTotal should count all secrets across multiple entries")

	// Check secretsRedactedByRule for each rule type
	require.NoError(t,
		testutil.GatherAndCompare(registry, strings.NewReader(`
			# HELP loki_secretfilter_secrets_redacted_by_rule_total Number of secrets redacted, partitioned by rule name.
			# TYPE loki_secretfilter_secrets_redacted_by_rule_total counter
			loki_secretfilter_secrets_redacted_by_rule_total{rule="grafana-api-key"} 2
			loki_secretfilter_secrets_redacted_by_rule_total{rule="gcp-api-key"} 1
		`),
			"loki_secretfilter_secrets_redacted_by_rule_total"))

	// Check secretsRedactedByOrigin values
	require.NoError(t,
		testutil.GatherAndCompare(registry, strings.NewReader(`
			# HELP loki_secretfilter_secrets_redacted_by_origin Number of secrets redacted, partitioned by origin label value.
			# TYPE loki_secretfilter_secrets_redacted_by_origin counter
			loki_secretfilter_secrets_redacted_by_origin{origin="test1"} 1
			loki_secretfilter_secrets_redacted_by_origin{origin="test2"} 1
			loki_secretfilter_secrets_redacted_by_origin{origin="test4"} 1
		`),
			"loki_secretfilter_secrets_redacted_by_origin"))
}

// TestArgumentsUpdate validates that the secretfilter component works correctly
// when its arguments are updated multiple times during runtime
func TestArgumentsUpdate(t *testing.T) {
	// Create a new registry to collect metrics
	registry := prometheus.NewRegistry()

	// Create a receiver to collect filtered logs
	ch1 := loki.NewLogsReceiver()

	// Initial arguments with basic configuration
	initialArgs := Arguments{
		ForwardTo:   []loki.LogsReceiver{ch1},
		OriginLabel: "",
	}

	// Create options with the test registry
	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
		Registerer:     registry,
	}

	// Create component with initial arguments
	c, err := New(opts, initialArgs)
	require.NoError(t, err)

	// Test updating the component with different arguments
	testData := []struct {
		description string
		args        Arguments
		inputLog    string
	}{
		{
			description: "Initial config - should redact secrets",
			args:        initialArgs,
			inputLog:    testLogs["grafana_api_key"].log,
		},
		{
			description: "Update 1 - Add origin label tracking",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "job",
			},
			inputLog: testLogs["gcp_api_key"].log,
		},
		{
			description: "Update 2 - Change origin label",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "instance",
			},
			inputLog: testLogs["stripe_key"].log,
		},
	}

	for _, td := range testData {
		t.Run(td.description, func(t *testing.T) {
			// Update the component with new arguments
			err := c.Update(td.args)
			require.NoError(t, err)

			// Create a test entry
			labels := model.LabelSet{
				"job":      "test-job",
				"instance": "test-instance",
			}
			entry := loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      td.inputLog,
				},
			}

			// Process the entry
			processedEntry := c.processEntry(entry)

			// Verify that redaction occurred
			require.NotEqual(t, entry.Line, processedEntry.Line, "Expected redaction to occur")
		})
	}
}
