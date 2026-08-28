package testappender_test

import (
	"fmt"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util/testappender"
)

func Example() {
	var app testappender.Appender
	app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar"), 60, 1234)
	app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
		Type: model.MetricTypeGauge,
	})

	expect := `
		# TYPE example_metric gauge
		example_metric{foo="bar"} 1234 60
	`

	_ = app.Commit()
	families, _ := app.MetricFamilies()

	err := testappender.Compare(families, expect)
	if err != nil {
		fmt.Println("Metrics do not match:", err)
	} else {
		fmt.Println("Metrics match!")
	}
	// Output: Metrics match!
}

// TestAppender_NoOp asserts that not doing anything results in no data.
func TestAppender_NoOp(t *testing.T) {
	var app testappender.Appender
	requireAppenderData(t, &app, "", false)
}

func TestAppender_Metadata(t *testing.T) {
	t.Run("No metadata", func(t *testing.T) {
		var app testappender.Appender
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar"), 60, 1234)

		expect := `
		# TYPE example_metric untyped
		example_metric{foo="bar"} 1234 60
	`
		requireAppenderData(t, &app, expect, false)
	})

	t.Run("Has metadata", func(t *testing.T) {
		var app testappender.Appender
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar"), 60, 1234)
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeGauge,
			Help: "example metric",
		})

		expect := `
		# HELP example_metric example metric
		# TYPE example_metric gauge
		example_metric{foo="bar"} 1234 60
		`
		requireAppenderData(t, &app, expect, false)
	})

	t.Run("No help field", func(t *testing.T) {
		var app testappender.Appender
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar"), 60, 1234)
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeGauge,
		})

		expect := `
		# TYPE example_metric gauge
		example_metric{foo="bar"} 1234 60
		`
		requireAppenderData(t, &app, expect, false)
	})
}

func TestAppender_Types(t *testing.T) {
	t.Run("Untyped", func(t *testing.T) {
		var app testappender.Appender
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar"), 60, 1234)
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeUnknown,
		})

		expect := `
		# TYPE example_metric untyped
		example_metric{foo="bar"} 1234 60
	`
		requireAppenderData(t, &app, expect, false)
	})

	t.Run("Counter", func(t *testing.T) {
		var app testappender.Appender
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar"), 60, 1234)
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeCounter,
		})

		expect := `
		# TYPE example_metric counter
		example_metric{foo="bar"} 1234 60
	`
		requireAppenderData(t, &app, expect, false)
	})

	t.Run("Gauge", func(t *testing.T) {
		var app testappender.Appender
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar"), 60, 1234)
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeGauge,
		})

		expect := `
		# TYPE example_metric gauge
		example_metric{foo="bar"} 1234 60
	`
		requireAppenderData(t, &app, expect, false)
	})

	t.Run("Summary", func(t *testing.T) {
		var app testappender.Appender

		// Summaries have quantiles from 0 to 1, counts, and sums. Append the
		// metadata first and then append all the various samples.
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeSummary,
		})

		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar", "quantile", "0"), 10, 10)
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar", "quantile", "0.25"), 10, 10)
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar", "quantile", "0.50"), 10, 10)
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar", "quantile", "0.75"), 10, 10)
		app.Append(0, labels.FromStrings("__name__", "example_metric", "foo", "bar", "quantile", "1"), 10, 10)
		app.Append(0, labels.FromStrings("__name__", "example_metric_count", "foo", "bar"), 10, 5)
		app.Append(0, labels.FromStrings("__name__", "example_metric_sum", "foo", "bar"), 10, 50)

		expect := `
		# TYPE example_metric summary
		example_metric{foo="bar",quantile="0"} 10 10
		example_metric{foo="bar",quantile="0.25"} 10 10
		example_metric{foo="bar",quantile="0.5"} 10 10
		example_metric{foo="bar",quantile="0.75"} 10 10
		example_metric{foo="bar",quantile="1"} 10 10
		example_metric_sum{foo="bar"} 50 10
		example_metric_count{foo="bar"} 5 10
	`
		requireAppenderData(t, &app, expect, false)
	})

	t.Run("Histogram", func(t *testing.T) {
		var app testappender.Appender

		// Histograms have buckets, counts, and sums. Append the metadata first and
		// then append all the various samples.
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeHistogram,
		})

		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "0.5"), 10, 10)
		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "0.75"), 10, 20)
		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "1"), 10, 30)
		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "+Inf"), 10, 40)
		app.Append(0, labels.FromStrings("__name__", "example_metric_count", "foo", "bar"), 10, 4)
		app.Append(0, labels.FromStrings("__name__", "example_metric_sum", "foo", "bar"), 10, 100)

		expect := `
		# TYPE example_metric histogram
		example_metric_bucket{foo="bar",le="0.5"} 10 10
		example_metric_bucket{foo="bar",le="0.75"} 20 10
		example_metric_bucket{foo="bar",le="1"} 30 10
		example_metric_bucket{foo="bar",le="+Inf"} 40 10
		example_metric_sum{foo="bar"} 100 10
		example_metric_count{foo="bar"} 4 10
	`
		requireAppenderData(t, &app, expect, false)
	})
}

func TestAppender_Exemplars(t *testing.T) {
	// These tests are the only tests where we explicitly test the OpenMetrics
	// exposition format since OpenMetrics is the only text exposition format
	// that supports exemplars.

	t.Run("Counter", func(t *testing.T) {
		var app testappender.Appender
		app.Append(0, labels.FromStrings("__name__", "example_metric_total", "foo", "bar"), 60, 1234)
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric_total"), metadata.Metadata{
			Type: model.MetricTypeCounter,
		})
		app.AppendExemplar(0, labels.FromStrings("__name__", "example_metric_total", "foo", "bar"), exemplar.Exemplar{
			Labels: labels.FromStrings("hello", "world"),
			Value:  1337,
			Ts:     30,
			HasTs:  true,
		})

		expect := `
		# TYPE example_metric counter
		example_metric_total{foo="bar"} 1234.0 0.06 # {hello="world"} 1337.0 0.03
		`
		requireAppenderData(t, &app, expect, true)
	})

	t.Run("Histogram", func(t *testing.T) {
		var app testappender.Appender

		// Histograms have buckets, counts, and sums. Append the metadata first and
		// then append all the various samples.
		app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric"), metadata.Metadata{
			Type: model.MetricTypeHistogram,
		})

		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "0.5"), 10, 10)
		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "0.75"), 10, 20)
		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "1"), 10, 30)
		app.Append(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "+Inf"), 10, 40)
		app.Append(0, labels.FromStrings("__name__", "example_metric_count", "foo", "bar"), 10, 4)
		app.Append(0, labels.FromStrings("__name__", "example_metric_sum", "foo", "bar"), 10, 100)

		// Exemplars must be attached to a specific bucket.
		app.AppendExemplar(0, labels.FromStrings("__name__", "example_metric_bucket", "foo", "bar", "le", "0.75"), exemplar.Exemplar{
			Labels: labels.FromStrings("hello", "world"),
			Value:  1337,
			Ts:     30,
			HasTs:  true,
		})

		expect := `
		# TYPE example_metric histogram
		example_metric_bucket{foo="bar",le="0.5"} 10 0.01
		example_metric_bucket{foo="bar",le="0.75"} 20 0.01 # {hello="world"} 1337.0 0.03
		example_metric_bucket{foo="bar",le="1.0"} 30 0.01
		example_metric_bucket{foo="bar",le="+Inf"} 40 0.01
		example_metric_sum{foo="bar"} 100.0 0.01
		example_metric_count{foo="bar"} 4 0.01
	`
		requireAppenderData(t, &app, expect, true)
	})
}

// TestAppender_MultipleMetrics tests that multiple metrics, where some have
// metadata and others don't, works as expected.
func TestAppender_MultipleMetrics(t *testing.T) {
	var app testappender.Appender

	app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric_a"), metadata.Metadata{
		Type: model.MetricTypeCounter,
	})
	app.UpdateMetadata(0, labels.FromStrings("__name__", "example_metric_b"), metadata.Metadata{
		Type: model.MetricTypeGauge,
	})

	app.Append(0, labels.FromStrings("__name__", "example_metric_a", "foo", "bar"), 10, 10)
	app.Append(0, labels.FromStrings("__name__", "example_metric_a", "foo", "rab"), 10, 20)
	app.Append(0, labels.FromStrings("__name__", "example_metric_b", "fizz", "buzz"), 10, 30)
	app.Append(0, labels.FromStrings("__name__", "example_metric_b", "fizz", "zzub"), 10, 40)
	app.Append(0, labels.FromStrings("__name__", "example_metric_c", "hello", "world"), 10, 50)
	app.Append(0, labels.FromStrings("__name__", "example_metric_c", "hello", "dlrow"), 10, 60)

	expect := `
		# TYPE example_metric_a counter
		example_metric_a{foo="bar"} 10 10
		example_metric_a{foo="rab"} 20 10
		# TYPE example_metric_b gauge
		example_metric_b{fizz="buzz"} 30 10
		example_metric_b{fizz="zzub"} 40 10
		# TYPE example_metric_c untyped
		example_metric_c{hello="dlrow"} 60 10
		example_metric_c{hello="world"} 50 10
	`
	requireAppenderData(t, &app, expect, false)
}

// requireAppenderData commits the appender and asserts that its resulting data
// matches the Prometheus Exposition Format string specified by expect.
func requireAppenderData(t *testing.T, app *testappender.Appender, expect string, openMetrics bool) {
	t.Helper()

	require.NoError(t, app.Commit(), "commit should not have failed")

	families, err := app.MetricFamilies()
	require.NoError(t, err, "failed to get metric families")

	var c testappender.Comparer
	c.OpenMetrics = openMetrics
	require.NoError(t, c.Compare(families, expect))
}
