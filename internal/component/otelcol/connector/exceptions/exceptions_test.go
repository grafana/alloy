package exceptions_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/grafana/alloy/internal/component/otelcol/connector/exceptions"
	"github.com/grafana/alloy/internal/component/otelcol/processor/processortest"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected exceptionsconnector.Config
		errorMsg string
	}{
		{
			testName: "Defaults",
			cfg: `
				output {}
			`,
			expected: exceptionsconnector.Config{
				Dimensions: []exceptionsconnector.Dimension{
					{Name: "exception.type"},
					{Name: "exception.message"},
				},
			},
		},
		{
			testName: "ExplicitValues",
			cfg: `
				dimension {
					name = "exception.type"
				}
				dimension {
					name    = "http.method"
					default = "GET"
				}
				exemplars {
					enabled = true
				}
				output {}
			`,
			expected: exceptionsconnector.Config{
				Dimensions: []exceptionsconnector.Dimension{
					{Name: "exception.type"},
					{Name: "http.method", Default: ptr.To("GET")},
				},
				Exemplars: exceptionsconnector.Exemplars{Enabled: true},
			},
		},
		{
			testName: "ReservedDimension",
			cfg: `
				dimension {
					name = "service.name"
				}
				output {}
			`,
			errorMsg: `duplicate dimension name "service.name"`,
		},
		{
			testName: "DuplicateDimension",
			cfg: `
				dimension {
					name = "http.method"
				}
				dimension {
					name = "http.method"
				}
				output {}
			`,
			errorMsg: `duplicate dimension name "http.method"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args exceptions.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*exceptionsconnector.Config)
			require.Equal(t, tc.expected, *actual)
		})
	}
}

const inputTrace = `{
	"resourceSpans": [{
		"resource": {
			"attributes": [{
				"key": "service.name",
				"value": { "stringValue": "shop-backend" }
			}]
		},
		"scopeSpans": [{
			"spans": [{
				"traceId": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
				"spanId": "086e83747d0e381e",
				"name": "GET /checkout",
				"kind": 2,
				"status": { "code": 2 },
				"attributes": [{
					"key": "http.method",
					"value": { "stringValue": "GET" }
				}],
				"events": [{
					"timeUnixNano": "1700000000000000000",
					"name": "exception",
					"attributes": [{
						"key": "exception.type",
						"value": { "stringValue": "NullPointerException" }
					},
					{
						"key": "exception.message",
						"value": { "stringValue": "cart is nil" }
					},
					{
						"key": "exception.stacktrace",
						"value": { "stringValue": "at checkout()" }
					}]
				}]
			}]
		}]
	}]
}`

const exceptionLog = `{
	"resourceLogs": [{
		"resource": {
			"attributes": [{
				"key": "service.name",
				"value": { "stringValue": "shop-backend" }
			}]
		},
		"scopeLogs": [{
			"logRecords": [{
				"timeUnixNano": "1700000000000000000",
				"severityNumber": 17,
				"severityText": "ERROR",
				"eventName": "exception",
				"traceId": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
				"spanId": "086e83747d0e381e",
				"attributes": [{
					"key": "http.method",
					"value": { "stringValue": "GET" }
				},
				{
					"key": "span.name",
					"value": { "stringValue": "GET /checkout" }
				},
				{
					"key": "span.kind",
					"value": { "stringValue": "SPAN_KIND_SERVER" }
				},
				{
					"key": "status.code",
					"value": { "stringValue": "STATUS_CODE_ERROR" }
				},
				{
					"key": "service.name",
					"value": { "stringValue": "shop-backend" }
				},
				{
					"key": "exception.type",
					"value": { "stringValue": "NullPointerException" }
				},
				{
					"key": "exception.message",
					"value": { "stringValue": "cart is nil" }
				},
				{
					"key": "exception.stacktrace",
					"value": { "stringValue": "at checkout()" }
				}]
			}]
		}]
	}]
}`

// inputLogs holds one exception log event and one regular log record.
const inputLogs = `{
	"resourceLogs": [{
		"resource": {
			"attributes": [{
				"key": "service.name",
				"value": { "stringValue": "shop-backend" }
			}]
		},
		"scopeLogs": [{
			"logRecords": [{
				"timeUnixNano": "1700000000000000000",
				"severityNumber": 17,
				"severityText": "ERROR",
				"eventName": "exception",
				"traceId": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
				"spanId": "086e83747d0e381e",
				"attributes": [{
					"key": "exception.type",
					"value": { "stringValue": "NullPointerException" }
				}]
			},
			{
				"timeUnixNano": "1700000000000000000",
				"severityNumber": 9,
				"severityText": "INFO",
				"body": { "stringValue": "checkout started" }
			}]
		}]
	}]
}`

const filteredLogs = `{
	"resourceLogs": [{
		"resource": {
			"attributes": [{
				"key": "service.name",
				"value": { "stringValue": "shop-backend" }
			}]
		},
		"scopeLogs": [{
			"logRecords": [{
				"timeUnixNano": "1700000000000000000",
				"severityNumber": 17,
				"severityText": "ERROR",
				"eventName": "exception",
				"traceId": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
				"spanId": "086e83747d0e381e",
				"attributes": [{
					"key": "exception.type",
					"value": { "stringValue": "NullPointerException" }
				}]
			}]
		}]
	}]
}`

const exceptionMetrics = `{
	"resourceMetrics": [{
		"scopeMetrics": [{
			"scope": { "name": "exceptionsconnector" },
			"metrics": [{
				"name": "exceptions",
				"sum": {
					"dataPoints": [{
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "shop-backend" }
						},
						{
							"key": "span.name",
							"value": { "stringValue": "GET /checkout" }
						},
						{
							"key": "span.kind",
							"value": { "stringValue": "SPAN_KIND_SERVER" }
						},
						{
							"key": "status.code",
							"value": { "stringValue": "STATUS_CODE_ERROR" }
						},
						{
							"key": "exception.type",
							"value": { "stringValue": "NullPointerException" }
						},
						{
							"key": "exception.message",
							"value": { "stringValue": "cart is nil" }
						}],
						"asInt": "1"
					}],
					"aggregationTemporality": 2,
					"isMonotonic": true
				}
			}]
		}]
	}]
}`

func Test_ComponentIO(t *testing.T) {
	const cfg = `
		output {
			// no-op: will be overridden by test code.
		}
	`

	tests := []struct {
		testName string
		signal   processortest.Signal
	}{
		{
			testName: "traces to logs",
			signal:   processortest.NewTraceToLogSignal(inputTrace, exceptionLog),
		},
		{
			testName: "logs to logs",
			signal:   processortest.NewLogSignal(inputLogs, filteredLogs),
		},
		{
			testName: "traces to metrics",
			signal:   processortest.NewTraceToMetricSignal(inputTrace, exceptionMetrics),
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			ctx := componenttest.TestContext(t)
			l := util.TestLogger(t)

			ctrl, err := componenttest.NewControllerFromID(l, "otelcol.connector.exceptions")
			require.NoError(t, err)

			var args exceptions.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

			// Override the arguments so signals get forwarded to the test channel.
			args.Output = tc.signal.MakeOutput()

			processortest.TestRunProcessor(processortest.ProcessorRunConfig{
				Ctx:        ctx,
				T:          t,
				Args:       args,
				TestSignal: tc.signal,
				Ctrl:       ctrl,
				L:          l,
			})
		})
	}
}
