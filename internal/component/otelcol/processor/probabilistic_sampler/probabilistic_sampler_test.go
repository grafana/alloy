package probabilistic_sampler_test

import (
	"context"
	"testing"

	probabilisticsampler "github.com/grafana/alloy/internal/component/otelcol/processor/probabilistic_sampler"
	"github.com/grafana/alloy/internal/component/otelcol/processor/processortest"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected probabilisticsamplerprocessor.Config
		errorMsg string
	}{
		{
			testName: "Defaults",
			cfg: `
					output {}
				`,
			expected: probabilisticsamplerprocessor.Config{
				SamplingPercentage: 0,
				SamplingPrecision:  4,
				HashSeed:           0,
				FailClosed:         true,
				AttributeSource:    "traceID",
				FromAttribute:      "",
				SamplingPriority:   "",
			},
		},
		{
			testName: "ExplicitValues",
			cfg: `
					sampling_percentage = 10
					hash_seed = 123
					mode = "equalizing"
					fail_closed = false
					sampling_precision = 13
					attribute_source = "record"
					from_attribute = "logID"
					sampling_priority = "priority"
					output {}
				`,
			expected: probabilisticsamplerprocessor.Config{
				SamplingPercentage: 10,
				HashSeed:           123,
				Mode:               "equalizing",
				FailClosed:         false,
				SamplingPrecision:  13,
				AttributeSource:    "record",
				FromAttribute:      "logID",
				SamplingPriority:   "priority",
			},
		},
		{
			testName: "Negative SamplingPercentage",
			cfg: `
					sampling_percentage = -1
					output {}
				`,
			errorMsg: "sampling rate is negative: -1.000000%",
		},
		{
			testName: "Invalid AttributeSource",
			cfg: `
					sampling_percentage = 0.1
					attribute_source = "example"
					output {}
				`,
			errorMsg: "invalid attribute source: example. Expected: traceID or record",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args probabilisticsampler.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.EqualError(t, err, tc.errorMsg)
				return
			}
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*probabilisticsamplerprocessor.Config)
			require.Equal(t, tc.expected, *actual)
		})
	}
}

func testRunProcessor(t *testing.T, processorConfig string, testSignal processortest.Signal) {
	ctx := componenttest.TestContext(t)
	testRunProcessorWithContext(ctx, t, processorConfig, testSignal)
}

func testRunProcessorWithContext(ctx context.Context, t *testing.T, processorConfig string, testSignal processortest.Signal) {
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.probabilistic_sampler")
	require.NoError(t, err)

	var args probabilisticsampler.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(processorConfig), &args))

	// Override the arguments so signals get forwarded to the test channel.
	args.Output = testSignal.MakeOutput()

	prc := processortest.ProcessorRunConfig{
		Ctx:        ctx,
		T:          t,
		Args:       args,
		TestSignal: testSignal,
		Ctrl:       ctrl,
		L:          l,
	}
	processortest.TestRunProcessor(prc)
}

func TestLogProcessing(t *testing.T) {
	cfg := `
			sampling_percentage = 100
			hash_seed = 123
			attribute_source = "traceID"
			from_attribute = "foo"
			output {
				// no-op: will be overridden by test code.
			}
		`
	var args probabilisticsampler.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	var inputLogs = `{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"attributes": [{
						"key": "foo",
						"value": {
							"stringValue": "bar"
						}
					}]
				}]
			}]
		}]
	}`

	var expectedOutputLogs = `{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"attributes": [{
						"key": "foo",
						"value": {
							"stringValue": "bar"
						}
					},
					{
						"key": "sampling.randomness",
						"value": {
							"stringValue": "37e0313b4df207"
						}
					},
					{
						"key": "sampling.threshold",
						"value": {
							"stringValue": "0"
						}
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewLogSignal(inputLogs, expectedOutputLogs))
}

func TestTraceProcessing(t *testing.T) {
	cfg := `
		sampling_percentage = 100
		hash_seed = 123
		output {
			// no-op: will be overridden by test code.
		}
	`

	var args probabilisticsampler.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	var inputTraces = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"traceId": "0123456789abcdef0123456789abcdef"
				}]
			}]
		}]
	}`

	expectedOutputTraces := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"traceId": "0123456789abcdef0123456789abcdef",
					"traceState": "ot=rv:db840a0a82091e;th:0"
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTraces, expectedOutputTraces))
}
