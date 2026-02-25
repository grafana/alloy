package secretfilter

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/secretfilter/testhelper"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
	"github.com/jaswdr/faker/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestSecretFiltering(t *testing.T) {
	// One component, one config load; all default cases run through it.
	RunTestCases(t, testhelper.TestConfigs["default"], DefaultTestCases())
}

// TestGitleaksConfig_InvalidPath checks that a missing config path returns an error.
// Valid custom config file loading (and [extend] useDefault) is tested in the
// extend package so it runs in a separate process and avoids gitleaks global state.
func TestGitleaksConfig_InvalidPath(t *testing.T) {
	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
	}
	args := Arguments{
		ForwardTo:      []loki.LogsReceiver{loki.NewLogsReceiver()},
		GitleaksConfig: filepath.Join(t.TempDir(), "nonexistent.gitleaks.toml"),
	}
	_, err := New(opts, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read gitleaks config")
}

func TestRedactPercent_FullRedaction(t *testing.T) {
	registry := prometheus.NewRegistry()
	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
		Registerer:     registry,
	}
	args := Arguments{
		ForwardTo:     []loki.LogsReceiver{loki.NewLogsReceiver()},
		RedactPercent: 100,
	}
	c, err := New(opts, args)
	require.NoError(t, err)

	entry := loki.Entry{
		Labels: model.LabelSet{},
		Entry:  push.Entry{Timestamp: time.Now(), Line: testhelper.TestLogs["grafana_api_key"].Log},
	}
	processed := c.processEntry(entry)
	require.Contains(t, processed.Entry.Line, "REDACTED", "expected full redaction to produce REDACTED placeholder")
	require.NotContains(t, processed.Entry.Line, testhelper.FakeSecrets["grafana-api-key"].Value)
}

func TestRedactPercent_Partial(t *testing.T) {
	registry := prometheus.NewRegistry()
	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
		Registerer:     registry,
	}
	args := Arguments{
		ForwardTo:     []loki.LogsReceiver{loki.NewLogsReceiver()},
		RedactPercent: 80,
	}
	c, err := New(opts, args)
	require.NoError(t, err)

	secret := testhelper.FakeSecrets["grafana-api-key"].Value
	entry := loki.Entry{
		Labels: model.LabelSet{},
		Entry:  push.Entry{Timestamp: time.Now(), Line: "log with secret " + secret + " end"},
	}
	processed := c.processEntry(entry)
	require.Contains(t, processed.Entry.Line, "...", "expected partial redaction to append ...")
	require.NotContains(t, processed.Entry.Line, secret, "original secret should not appear in full")
	// First 20% of secret should be present (gitleaks Redact(80) keeps leading 20% + "...")
	leadingLen := len(secret) * 20 / 100
	if leadingLen > 0 {
		require.Contains(t, processed.Entry.Line, secret[:leadingLen], "expected leading portion of secret to remain")
	}
}

func TestRedactWith_CustomPlaceholder(t *testing.T) {
	registry := prometheus.NewRegistry()
	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
		Registerer:     registry,
	}
	args := Arguments{
		ForwardTo:  []loki.LogsReceiver{loki.NewLogsReceiver()},
		RedactWith: "***REDACTED***",
	}
	c, err := New(opts, args)
	require.NoError(t, err)

	entry := loki.Entry{
		Labels: model.LabelSet{},
		Entry:  push.Entry{Timestamp: time.Now(), Line: testhelper.TestLogs["gcp_api_key"].Log},
	}
	processed := c.processEntry(entry)
	require.Contains(t, processed.Entry.Line, "***REDACTED***")
	require.NotContains(t, processed.Entry.Line, testhelper.FakeSecrets["gcp-api-key"].Value)
}

// TestDefaultRedactPercent_usesEighty verifies that with no redact_with and no redact_percent,
// the component defaults to 80% redaction (gitleaks-style: leading 20% + "...").
func TestDefaultRedactPercent_usesEighty(t *testing.T) {
	registry := prometheus.NewRegistry()
	opts := component.Options{
		Logger:         util.TestLogger(t),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
		Registerer:     registry,
	}
	args := Arguments{
		ForwardTo: []loki.LogsReceiver{loki.NewLogsReceiver()},
		// RedactWith and RedactPercent left at zero values => effective 80%
	}
	c, err := New(opts, args)
	require.NoError(t, err)

	secret := testhelper.FakeSecrets["grafana-api-key"].Value
	entry := loki.Entry{
		Labels: model.LabelSet{},
		Entry:  push.Entry{Timestamp: time.Now(), Line: "log " + secret + " end"},
	}
	processed := c.processEntry(entry)
	require.Contains(t, processed.Entry.Line, "...", "default 80%% redaction should append ...")
	require.NotContains(t, processed.Entry.Line, secret, "original secret should not appear in full")
}

func BenchmarkAllTypesNoSecret(b *testing.B) {
	// Run benchmarks with no secrets in the logs, with all regexes enabled
	runBenchmarks(b, testhelper.TestConfigs["default"], 0, "")
}

func BenchmarkAllTypesWithSecret(b *testing.B) {
	// Run benchmarks with secrets in the logs (20% of log entries), with all regexes enabled
	runBenchmarks(b, testhelper.TestConfigs["default"], 20, "gcp_api_key")
}

func BenchmarkAllTypesWithLotsOfSecrets(b *testing.B) {
	// Run benchmarks with secrets in the logs (80% of log entries), with all regexes enabled
	runBenchmarks(b, testhelper.TestConfigs["default"], 80, "gcp_api_key")
}

func runBenchmarks(b *testing.B, config string, percentageSecrets int, secretName string) {
	ch1 := loki.NewLogsReceiver()
	var args Arguments
	require.NoError(b, syntax.Unmarshal([]byte(config), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}

	opts := component.Options{
		Logger:         &noopLogger{}, // Disable logging so that it keeps a clean benchmark output
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
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
			middleStr = testhelper.TestLogs[secretName].Log
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
	for _, testLog := range testhelper.TestLogs {
		f.Add(testLog.Log)
	}

	opts := component.Options{
		Logger:         util.TestLogger(f),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: testhelper.GetServiceData,
	}
	ch1 := loki.NewLogsReceiver()

	// Create component with default config
	var args Arguments
	require.NoError(f, syntax.Unmarshal([]byte(testhelper.TestConfigs["default"]), &args))
	args.ForwardTo = []loki.LogsReceiver{ch1}
	c, err := New(opts, args)
	require.NoError(f, err)

	f.Fuzz(func(t *testing.T, log string) {
		entry := loki.Entry{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Now(), Line: log}}
		c.processEntry(entry)
	})
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
			inputLog:              testhelper.TestLogs["no_secret"].Log,
			expectedRedactedTotal: 0,
		},
		{
			name:                  "Single Grafana API key secret",
			inputLog:              testhelper.TestLogs["grafana_api_key"].Log,
			expectedRedactedTotal: 1,
			expectedRedactedByRule: map[string]int{
				"grafana-api-key": 1,
			},
		},
		{
			name:                  "Multiple secrets",
			inputLog:              testhelper.TestLogs["multiple_secrets"].Log,
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
				GetServiceData: testhelper.GetServiceData,
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
		GetServiceData: testhelper.GetServiceData,
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
		GetServiceData: testhelper.GetServiceData,
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
				Line:      testhelper.TestLogs["grafana_api_key"].Log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test2"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testhelper.TestLogs["gcp_api_key"].Log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test3"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testhelper.TestLogs["no_secret"].Log,
			},
		},
		{
			Labels: model.LabelSet{"job": "test4"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      testhelper.TestLogs["grafana_api_key"].Log,
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
		GetServiceData: testhelper.GetServiceData,
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
			inputLog:    testhelper.TestLogs["grafana_api_key"].Log,
		},
		{
			description: "Update 1 - Add origin label tracking",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "job",
			},
			inputLog: testhelper.TestLogs["gcp_api_key"].Log,
		},
		{
			description: "Update 2 - Change origin label",
			args: Arguments{
				ForwardTo:   []loki.LogsReceiver{ch1},
				OriginLabel: "instance",
			},
			inputLog: testhelper.TestLogs["stripe_key"].Log,
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
