package deps

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
	// Both must match manifests/mimir.yaml.
	mimirSelector = "app=mimir"
	mimirHTTPPort = "9009"
)

//go:embed manifests/mimir.yaml
var mimirManifest string

type metricsResponse struct {
	Status string `json:"status"`
	Data   []struct {
		Name string `json:"__name__"`
	} `json:"data"`
}

type metadataResponse struct {
	Status string                        `json:"status"`
	Data   map[string][]ExpectedMetadata `json:"data"`
}

type instantQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric    map[string]string `json:"metric"`
			Value     []any             `json:"value"`
			Histogram []any             `json:"histogram"`
		} `json:"result"`
	} `json:"data"`
}

type seriesResponse struct {
	Status string              `json:"status"`
	Data   []map[string]string `json:"data"`
}

// ExpectedMetadata is both the JSON payload shape from Mimir's metadata
// endpoint and the input to QueryMetadata. Empty fields are not asserted.
type ExpectedMetadata struct {
	Type string `json:"type"`
	Help string `json:"help"`
	Unit string `json:"unit"`
}

// Mimir runs a single-pod Mimir in monolithic mode (filesystem storage,
// in-memory rings). In-cluster URL: http://mimir:9009.
type Mimir struct {
	opts            MimirOptions
	namespace       string
	localPort       string
	stopPortForward func()
	installed       bool
}

type MimirOptions struct {
	Namespace string
}

func NewMimir(opts MimirOptions) *Mimir {
	return &Mimir{opts: opts, namespace: opts.Namespace}
}

func (m *Mimir) Name() string { return "mimir" }

func (m *Mimir) Install(ctx *harness.TestContext) error {
	if m.namespace == "" {
		return fmt.Errorf("mimir namespace is required")
	}

	if err := util.Step("apply mimir manifest", func() error {
		return harness.ApplyManifest(m.namespace, mimirManifest)
	}); err != nil {
		return err
	}
	m.installed = true

	if err := util.Step("wait for mimir pod ready", func() error {
		return harness.WaitForReady(m.namespace, mimirSelector)
	}); err != nil {
		return err
	}

	localPort, stop, err := startPortForwardWithRetries(m.namespace, "mimir", 5, mimirHTTPPort)
	if err != nil {
		return err
	}
	m.localPort = localPort
	m.stopPortForward = stop
	return nil
}

func (m *Mimir) Cleanup() {
	if m.stopPortForward != nil {
		m.stopPortForward()
	}
	if !m.installed || m.namespace == "" {
		return
	}
	_ = harness.DeleteManifest(m.namespace, mimirManifest)
}

// QueryMetrics polls Mimir for series labelled with alloy_test_name=testName
// and asserts every expected metric name is present.
func (m *Mimir) QueryMetrics(t *testing.T, testName string, expectedMetrics []string) {
	t.Helper()
	mimirURL := m.endpoint("/prometheus/api/v1/")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(mimirURL + "series")
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("match[]", "{"+testNameLabel+"=\""+testName+"\"}")
		queryURL.RawQuery = values.Encode()
		resp := curl(c, queryURL.String(), nil)

		var parsed metricsResponse
		err = json.Unmarshal([]byte(resp), &parsed)
		require.NoError(c, err, "failed to parse mimir response: %s", resp)
		require.Equal(c, "success", parsed.Status, "mimir query failed: %s", resp)

		actualMetrics := make(map[string]struct{}, len(parsed.Data))
		for _, metric := range parsed.Data {
			actualMetrics[metric.Name] = struct{}{}
		}

		var missingMetrics []string
		for _, expectedMetric := range expectedMetrics {
			if _, exists := actualMetrics[expectedMetric]; !exists {
				missingMetrics = append(missingMetrics, expectedMetric)
			}
		}

		require.Emptyf(c, missingMetrics, "missing expected metrics for %s=%s: %v found=%v", testNameLabel, testName, missingMetrics, actualMetrics)
	}, timeout, retryInterval)
}

// QueryPositive polls an instant query for each metric (scoped to testName) and
// asserts at least one returned sample is greater than zero. It checks a metric
// is not just present but reports real data.
func (m *Mimir) QueryPositive(t *testing.T, testName string, metrics []string) {
	t.Helper()
	base := m.endpoint("/prometheus/api/v1/query")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, metric := range metrics {
			queryURL, err := url.Parse(base)
			require.NoError(c, err)
			values := queryURL.Query()
			values.Set("query", metric+"{"+testNameLabel+"=\""+testName+"\"}")
			queryURL.RawQuery = values.Encode()
			resp := curl(c, queryURL.String(), nil)

			var parsed instantQueryResponse
			require.NoError(c, json.Unmarshal([]byte(resp), &parsed), "failed to parse query response: %s", resp)
			require.Equal(c, "success", parsed.Status, "mimir query failed: %s", resp)
			require.NotEmptyf(c, parsed.Data.Result, "%s: no samples for %s=%s", metric, testNameLabel, testName)

			var maxValue float64
			for _, result := range parsed.Data.Result {
				if len(result.Value) != 2 {
					continue
				}
				sample, ok := result.Value[1].(string)
				if !ok {
					continue
				}
				value, convErr := strconv.ParseFloat(sample, 64)
				if convErr == nil && value > maxValue {
					maxValue = value
				}
			}
			require.Greaterf(c, maxValue, 0.0, "%s: expected a positive sample", metric)
		}
	}, timeout, retryInterval)
}

// HistogramSample is one native histogram sample returned by an instant query.
type HistogramSample struct {
	Labels  map[string]string
	Count   float64
	Sum     float64
	Buckets []HistogramBucket
}

// Name returns the sample's metric name.
func (h HistogramSample) Name() string { return h.Labels["__name__"] }

// HistogramBucket is one bucket of a native histogram. BoundaryRule follows
// Prometheus' encoding: 0 open both ends, 1 closed left, 2 closed right,
// 3 closed both ends.
type HistogramBucket struct {
	BoundaryRule int
	Lower        float64
	Upper        float64
	Count        float64
}

// QueryHistograms polls an instant query for each metric (scoped to testName)
// and asserts it arrives as a native histogram rather than a float sample. It
// then calls assertSample for every returned sample so callers decide what to
// check. Assertions are retried, so use the supplied CollectT rather than t.
func (m *Mimir) QueryHistograms(t *testing.T, testName string, metrics []string, assertSample func(c *assert.CollectT, sample HistogramSample)) {
	t.Helper()
	base := m.endpoint("/prometheus/api/v1/query")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, metric := range metrics {
			queryURL, err := url.Parse(base)
			require.NoError(c, err)
			values := queryURL.Query()
			values.Set("query", metric+"{"+testNameLabel+"=\""+testName+"\"}")
			queryURL.RawQuery = values.Encode()
			resp := curl(c, queryURL.String(), nil)

			var parsed instantQueryResponse
			require.NoError(c, json.Unmarshal([]byte(resp), &parsed), "failed to parse query response: %s", resp)
			require.Equal(c, "success", parsed.Status, "mimir query failed: %s", resp)
			require.NotEmptyf(c, parsed.Data.Result, "%s: no samples for %s=%s", metric, testNameLabel, testName)

			for _, result := range parsed.Data.Result {
				require.Emptyf(c, result.Value, "%s: expected a native histogram, got a float sample", metric)
				sample, convErr := toHistogramSample(result.Metric, result.Histogram)
				require.NoErrorf(c, convErr, "%s: %v", metric, convErr)
				assertSample(c, sample)
			}
		}
	}, timeout, retryInterval)
}

func toHistogramSample(labels map[string]string, raw []any) (HistogramSample, error) {
	if len(raw) != 2 {
		return HistogramSample{}, fmt.Errorf("expected a native histogram sample, got %d fields", len(raw))
	}
	data, ok := raw[1].(map[string]any)
	if !ok {
		return HistogramSample{}, fmt.Errorf("unexpected histogram shape %T", raw[1])
	}

	sample := HistogramSample{Labels: labels}
	var err error
	if sample.Count, err = numericField(data, "count"); err != nil {
		return HistogramSample{}, err
	}
	if sample.Sum, err = numericField(data, "sum"); err != nil {
		return HistogramSample{}, err
	}

	buckets, _ := data["buckets"].([]any)
	for i, rawBucket := range buckets {
		fields, isSlice := rawBucket.([]any)
		if !isSlice || len(fields) != 4 {
			return HistogramSample{}, fmt.Errorf("bucket %d: expected 4 fields, got %v", i, rawBucket)
		}
		bucket := HistogramBucket{}
		for j, target := range []*float64{nil, &bucket.Lower, &bucket.Upper, &bucket.Count} {
			if j == 0 {
				rule, isNumber := fields[0].(float64)
				if !isNumber {
					return HistogramSample{}, fmt.Errorf("bucket %d: boundary rule is %T", i, fields[0])
				}
				bucket.BoundaryRule = int(rule)
				continue
			}
			text, isString := fields[j].(string)
			if !isString {
				return HistogramSample{}, fmt.Errorf("bucket %d field %d: expected a string, got %T", i, j, fields[j])
			}
			if *target, err = strconv.ParseFloat(text, 64); err != nil {
				return HistogramSample{}, fmt.Errorf("bucket %d field %d: %w", i, j, err)
			}
		}
		sample.Buckets = append(sample.Buckets, bucket)
	}

	return sample, nil
}

func numericField(data map[string]any, field string) (float64, error) {
	text, ok := data[field].(string)
	if !ok {
		return 0, fmt.Errorf("missing histogram %s", field)
	}
	return strconv.ParseFloat(text, 64)
}

// QueryMetricWithLabelsPresent asserts that metricName has at least one series
// for testName carrying every given label with a non-empty value. Passing
// multiple labels requires them on the same series. Naming the metric confirms
// the labels are attached to a real cAdvisor metric, not just any series.
func (m *Mimir) QueryMetricWithLabelsPresent(t *testing.T, testName, metricName string, labelNames ...string) {
	t.Helper()

	matchers := testNameLabel + "=\"" + testName + "\""
	for _, labelName := range labelNames {
		matchers += "," + labelName + "=~\".+\""
	}
	selector := metricName + "{" + matchers + "}"

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(m.endpoint("/prometheus/api/v1/series"))
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("match[]", selector)
		queryURL.RawQuery = values.Encode()
		resp := curl(c, queryURL.String(), nil)

		var parsed seriesResponse
		require.NoError(c, json.Unmarshal([]byte(resp), &parsed), "failed to parse series response: %s", resp)
		require.Equal(c, "success", parsed.Status, "mimir series query failed: %s", resp)
		require.NotEmptyf(c, parsed.Data, "no %s series carrying labels %v for %s=%s", metricName, labelNames, testNameLabel, testName)
	}, timeout, retryInterval)
}

// QueryMetadata asserts each expected metric appears in Mimir's
// /api/v1/metadata with the requested Type/Help/Unit.
func (m *Mimir) QueryMetadata(t *testing.T, expected map[string]ExpectedMetadata) {
	t.Helper()
	endpoint := m.endpoint("/prometheus/api/v1/metadata")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp := curl(c, endpoint, nil)

		var parsed metadataResponse
		err := json.Unmarshal([]byte(resp), &parsed)
		require.NoError(c, err, "failed to parse mimir metadata response: %s", resp)
		require.Equal(c, "success", parsed.Status, "mimir metadata query failed: %s", resp)

		var missing []string
		var mismatched []string
		for name, want := range expected {
			entries, ok := parsed.Data[name]
			if !ok || len(entries) == 0 {
				missing = append(missing, name)
				continue
			}
			if len(entries) != 1 {
				mismatched = append(mismatched, fmt.Sprintf("%s: want exactly 1 metadata entry, got %d", name, len(entries)))
				continue
			}
			got := entries[0]
			if want.Type != "" && got.Type != want.Type {
				mismatched = append(mismatched, fmt.Sprintf("%s: type want=%q got=%q", name, want.Type, got.Type))
			}
			if want.Help != "" && got.Help != want.Help {
				mismatched = append(mismatched, fmt.Sprintf("%s: help want=%q got=%q", name, want.Help, got.Help))
			}
			if want.Unit != "" && got.Unit != want.Unit {
				mismatched = append(mismatched, fmt.Sprintf("%s: unit want=%q got=%q", name, want.Unit, got.Unit))
			}
		}

		require.Emptyf(c, missing, "missing metadata for metrics: %v", missing)
		require.Emptyf(c, mismatched, "metadata mismatches: %v", mismatched)
	}, timeout, retryInterval)
}

func (m *Mimir) CheckAlertsConfig(t *testing.T, expectedFile string) {
	t.Helper()
	expectedMimirConfigBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	expectedMimirConfig := string(expectedMimirConfigBytes)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := curl(c, m.endpoint("/api/v1/alerts"), nil)
		require.Equal(c, expectedMimirConfig, actualMimirConfig)
	}, timeout, retryInterval)
}

func (m *Mimir) endpoint(path string) string {
	return "http://localhost:" + m.localPort + path
}
