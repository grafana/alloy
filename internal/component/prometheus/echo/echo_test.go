package echo

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestArgumentsDefaults(t *testing.T) {
	args := Arguments{}
	args.SetToDefault()
	require.Equal(t, DefaultArguments, args)
}

func TestComponent_Creation(t *testing.T) {
	ctx := componenttest.TestContext(t)

	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logging.NewSlogNop(),
	}, Arguments{})
	require.NoError(t, err)
	require.NotNil(t, comp)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	time.Sleep(100 * time.Millisecond)
}

func TestComponent_ExportsReceiver(t *testing.T) {
	var exports component.Exports

	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logging.NewSlogNop(),
		OnStateChange: func(e component.Exports) {
			exports = e
		},
	}, Arguments{})
	require.NoError(t, err)
	require.NotNil(t, comp)

	echoExports, ok := exports.(Exports)
	require.True(t, ok)
	require.NotNil(t, echoExports.Receiver)

	require.Equal(t, comp, echoExports.Receiver)
}

func TestAppender_BasicMetrics(t *testing.T) {
	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logging.NewSlogNop(),
	}, Arguments{})
	require.NoError(t, err)

	ctx := context.Background()
	appender := comp.Appender(ctx)
	require.NotNil(t, appender)

	lbls := labels.FromStrings("__name__", "test_metric", "job", "test")

	ref, err := appender.Append(0, lbls, time.Now().Unix(), 42.0)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(0), ref)

	err = appender.Commit()
	require.NoError(t, err)
}

func TestAppender_Rollback(t *testing.T) {
	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logging.NewSlogNop(),
	}, Arguments{})
	require.NoError(t, err)

	ctx := context.Background()
	appender := comp.Appender(ctx)
	require.NotNil(t, appender)

	lbls := labels.FromStrings("__name__", "test_metric", "job", "test")

	_, err = appender.Append(0, lbls, time.Now().Unix(), 42.0)
	require.NoError(t, err)

	err = appender.Rollback()
	require.NoError(t, err)
}

func TestAppender_MultipleMetrics(t *testing.T) {
	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logging.NewSlogNop(),
	}, Arguments{})
	require.NoError(t, err)

	ctx := context.Background()
	appender := comp.Appender(ctx)
	require.NotNil(t, appender)

	metrics := []struct {
		name  string
		value float64
	}{
		{"metric_one", 1.0},
		{"metric_two", 2.0},
		{"metric_three", 3.0},
	}

	for _, metric := range metrics {
		lbls := labels.FromStrings("__name__", metric.name, "job", "test")
		_, err = appender.Append(0, lbls, time.Now().Unix(), metric.value)
		require.NoError(t, err)
	}

	err = appender.Commit()
	require.NoError(t, err)
}

func TestComponent_Update(t *testing.T) {
	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logging.NewSlogNop(),
	}, Arguments{})
	require.NoError(t, err)

	err = comp.Update(Arguments{})
	require.NoError(t, err)
}

func TestAppender_WithExpfmtEncoding(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logging.New(&buf, logging.Options{
		Level:  logging.LevelInfo,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logger.Slog(),
	}, Arguments{})
	require.NoError(t, err)

	ctx := context.Background()
	appender := comp.Appender(ctx)

	lbls := labels.FromStrings("__name__", "test_metric", "job", "test_job", "instance", "localhost:8080")
	_, err = appender.Append(0, lbls, time.Now().Unix(), 42.0)
	require.NoError(t, err)

	err = appender.Commit()
	require.NoError(t, err)

	loggedOutput := buf.String()
	require.NotEmpty(t, loggedOutput)
	require.Contains(t, loggedOutput, "test_metric")
	require.Contains(t, loggedOutput, "job=\\\"test_job\\\"")
	require.Contains(t, loggedOutput, "instance=\\\"localhost:8080\\\"")
	require.Contains(t, loggedOutput, "42")

	require.NotContains(t, loggedOutput, "Prometheus metrics received by")
	require.NotContains(t, loggedOutput, "Timestamp:")
}

func TestAppender_WithOpenMetricsFormat(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logging.New(&buf, logging.Options{
		Level:  logging.LevelInfo,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	args := Arguments{Format: "openmetrics"}

	comp, err := New(component.Options{
		ID:      "test",
		SLogger: logger.Slog(),
	}, args)
	require.NoError(t, err)

	ctx := context.Background()
	appender := comp.Appender(ctx)

	lbls := labels.FromStrings("__name__", "test_metric", "job", "test_job")
	_, err = appender.Append(0, lbls, time.Now().Unix(), 42.0)
	require.NoError(t, err)

	err = appender.Commit()
	require.NoError(t, err)

	loggedOutput := buf.String()

	require.NotEmpty(t, loggedOutput)
	require.Contains(t, loggedOutput, "test_metric")
	require.Contains(t, loggedOutput, "job=\\\"test_job\\\"")

	t.Logf("OpenMetrics output: %s", loggedOutput)
}

func TestArguments_Defaults(t *testing.T) {
	args := Arguments{}
	args.SetToDefault()

	require.Equal(t, "text", args.Format)
}
