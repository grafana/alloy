package processortest

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

//
// Utilities for running a processor end-to-end and testing its outputs.
// They work for metrics, logs and traces.
//

type Signal interface {
	MakeOutput() *otelcol.ConsumerArguments
	ConsumeInput(ctx context.Context, consumer otelcol.Consumer) error
	CheckOutput(t *testing.T)
}

type ProcessorRunConfig struct {
	Ctx                   context.Context
	T                     *testing.T
	Args                  component.Arguments
	TestSignal            Signal
	AdditionalSignalSends int
	Ctrl                  *componenttest.Controller
	L                     log.Logger
}

func TestRunProcessor(c ProcessorRunConfig) {
	go func() {
		err := c.Ctrl.Run(c.Ctx, c.Args)
		require.NoError(c.T, err)
	}()

	require.NoError(c.T, c.Ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(c.T, c.Ctrl.WaitExports(time.Second), "component never exported anything")

	// Send signals in the background to our processor.
	go func() {
		exports := c.Ctrl.Exports().(otelcol.ConsumerExports)

		bo := backoff.New(c.Ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			for i := 0; i <= c.AdditionalSignalSends; i++ {
				err := c.TestSignal.ConsumeInput(c.Ctx, exports.Input)
				if err != nil {
					level.Error(c.L).Log("msg", "failed to send signal", "err", err)
					bo.Wait()
					continue
				}
			}
			return
		}
	}()

	c.TestSignal.CheckOutput(c.T)
}

//
// Trace to Logs
//

type traceToLogSignal struct {
	logCh             chan plog.Logs
	inputTrace        ptrace.Traces
	expectedOutputLog plog.Logs
}

func NewTraceToLogSignal(inputJson string, expectedOutputJson string) Signal {
	return &traceToLogSignal{
		logCh:             make(chan plog.Logs),
		inputTrace:        CreateTestTraces(inputJson),
		expectedOutputLog: CreateTestLogs(expectedOutputJson),
	}
}

func (s traceToLogSignal) MakeOutput() *otelcol.ConsumerArguments {
	return makeLogsOutput(s.logCh)
}

func (s traceToLogSignal) ConsumeInput(ctx context.Context, consumer otelcol.Consumer) error {
	return consumer.ConsumeTraces(ctx, s.inputTrace)
}

func (s traceToLogSignal) CheckOutput(t *testing.T) {
	// Wait for our processor to finish and forward data to logCh.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for logs")
	case actualLog := <-s.logCh:
		CompareLogs(t, s.expectedOutputLog, actualLog)
	}
}

//
// Trace to Metrics
//

type traceToMetricSignal struct {
	metricCh             chan pmetric.Metrics
	inputTrace           ptrace.Traces
	expectedOutputMetric pmetric.Metrics
}

// Any timestamps inside expectedOutputJson should be set to 0.
func NewTraceToMetricSignal(inputJson string, expectedOutputJson string) Signal {
	return &traceToMetricSignal{
		metricCh:             make(chan pmetric.Metrics),
		inputTrace:           CreateTestTraces(inputJson),
		expectedOutputMetric: CreateTestMetrics(expectedOutputJson),
	}
}

func (s traceToMetricSignal) MakeOutput() *otelcol.ConsumerArguments {
	return makeMetricsOutput(s.metricCh)
}

func (s traceToMetricSignal) ConsumeInput(ctx context.Context, consumer otelcol.Consumer) error {
	return consumer.ConsumeTraces(ctx, s.inputTrace)
}

// Wait for the component to finish and check its output.
func (s traceToMetricSignal) CheckOutput(t *testing.T) {
	// Set the timeout to a few seconds so that all components have finished.
	// Components such as otelcol.connector.spanmetrics may need a few
	// seconds before they output metrics.
	timeoutCh := time.After(time.Second * 5)
	latestReceived := ""

	for {
		select {
		case <-timeoutCh:
			require.FailNow(t, "failed waiting for the required metrics", "latest received: %s", latestReceived)
			return
		case actualMetric := <-s.metricCh:
			jsonMarshaler := pmetric.JSONMarshaler{}
			metricsBuf, err := jsonMarshaler.MarshalMetrics(actualMetric)
			require.NoError(t, err)
			metricsStr := string(metricsBuf)
			t.Logf("actual metrics: %s", metricsStr)
			latestReceived = metricsStr

			err = CompareMetrics(t, s.expectedOutputMetric, actualMetric)
			if err == nil {
				// The expected metrics were received - return.
				t.Logf("received the expected metric")
				return
			}
			t.Logf("received a metric which is NOT the expected one")
		}
	}
}

//
// Traces
//

type traceSignal struct {
	traceCh             chan ptrace.Traces
	inputTrace          ptrace.Traces
	expectedOutputTrace ptrace.Traces
}

func NewTraceSignal(inputJson string, expectedOutputJson string) Signal {
	return &traceSignal{
		traceCh:             make(chan ptrace.Traces),
		inputTrace:          CreateTestTraces(inputJson),
		expectedOutputTrace: CreateTestTraces(expectedOutputJson),
	}
}

func (s traceSignal) MakeOutput() *otelcol.ConsumerArguments {
	return makeTracesOutput(s.traceCh)
}

func (s traceSignal) ConsumeInput(ctx context.Context, consumer otelcol.Consumer) error {
	return consumer.ConsumeTraces(ctx, s.inputTrace)
}

func (s traceSignal) CheckOutput(t *testing.T) {
	// Wait for our processor to finish and forward data to traceCh.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for traces")
	case actualTrace := <-s.traceCh:
		CompareTraces(t, s.expectedOutputTrace, actualTrace)
	}
}

// traceJson should match format from the protobuf definition:
// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto
func CreateTestTraces(traceJson string) ptrace.Traces {
	decoder := &ptrace.JSONUnmarshaler{}
	data, err := decoder.UnmarshalTraces([]byte(traceJson))
	if err != nil {
		panic(err)
	}
	return data
}

// makeTracesOutput returns ConsumerArguments which will forward traces to the
// provided channel.
func makeTracesOutput(ch chan ptrace.Traces) *otelcol.ConsumerArguments {
	traceConsumer := fakeconsumer.Consumer{
		ConsumeTracesFunc: func(ctx context.Context, t ptrace.Traces) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- t:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Traces: []otelcol.Consumer{&traceConsumer},
	}
}

//
// Logs
//

type logSignal struct {
	logCh             chan plog.Logs
	inputLog          plog.Logs
	expectedOutputLog plog.Logs
}

func NewLogSignal(inputJson string, expectedOutputJson string) Signal {
	return &logSignal{
		logCh:             make(chan plog.Logs),
		inputLog:          CreateTestLogs(inputJson),
		expectedOutputLog: CreateTestLogs(expectedOutputJson),
	}
}

func (s logSignal) MakeOutput() *otelcol.ConsumerArguments {
	return makeLogsOutput(s.logCh)
}

func (s logSignal) ConsumeInput(ctx context.Context, consumer otelcol.Consumer) error {
	return consumer.ConsumeLogs(ctx, s.inputLog)
}

func (s logSignal) CheckOutput(t *testing.T) {
	// Wait for our processor to finish and forward data to logCh.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for logs")
	case actualLog := <-s.logCh:
		CompareLogs(t, s.expectedOutputLog, actualLog)
	}
}

// makeLogsOutput returns ConsumerArguments which will forward logs to the
// provided channel.
func makeLogsOutput(ch chan plog.Logs) *otelcol.ConsumerArguments {
	logConsumer := fakeconsumer.Consumer{
		ConsumeLogsFunc: func(ctx context.Context, t plog.Logs) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- t:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Logs: []otelcol.Consumer{&logConsumer},
	}
}

// logJson should match format from the protobuf definition:
// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/logs/v1/logs.proto
func CreateTestLogs(logJson string) plog.Logs {
	decoder := &plog.JSONUnmarshaler{}
	data, err := decoder.UnmarshalLogs([]byte(logJson))
	if err != nil {
		panic(err)
	}
	return data
}

//
// Metrics
//

type metricSignal struct {
	metricCh             chan pmetric.Metrics
	inputMetric          pmetric.Metrics
	expectedOutputMetric pmetric.Metrics
}

func NewMetricSignal(inputJson string, expectedOutputJson string) Signal {
	return &metricSignal{
		metricCh:             make(chan pmetric.Metrics),
		inputMetric:          CreateTestMetrics(inputJson),
		expectedOutputMetric: CreateTestMetrics(expectedOutputJson),
	}
}

func (s metricSignal) MakeOutput() *otelcol.ConsumerArguments {
	return makeMetricsOutput(s.metricCh)
}

func (s metricSignal) ConsumeInput(ctx context.Context, consumer otelcol.Consumer) error {
	return consumer.ConsumeMetrics(ctx, s.inputMetric)
}

func (s metricSignal) CheckOutput(t *testing.T) {
	// Wait for our processor to finish and forward data to logCh.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for logs")
	case actualMetric := <-s.metricCh:
		err := CompareMetrics(t, s.expectedOutputMetric, actualMetric)
		require.NoError(t, err)
	}
}

// makeMetricsOutput returns ConsumerArguments which will forward metrics to the
// provided channel.
func makeMetricsOutput(ch chan pmetric.Metrics) *otelcol.ConsumerArguments {
	metricConsumer := fakeconsumer.Consumer{
		ConsumeMetricsFunc: func(ctx context.Context, t pmetric.Metrics) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- t:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Metrics: []otelcol.Consumer{&metricConsumer},
	}
}

// metricJson should match format from the protobuf definition:
// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto
func CreateTestMetrics(metricJson string) pmetric.Metrics {
	decoder := &pmetric.JSONUnmarshaler{}
	data, err := decoder.UnmarshalMetrics([]byte(metricJson))
	if err != nil {
		panic(err)
	}
	return data
}
