package spanlogs_test

import (
	"context"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/connector/spanlogs"
	"github.com/grafana/alloy/internal/component/otelcol/processor/processortest"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func testRunProcessor(t *testing.T, processorConfig string, testSignal processortest.Signal) {
	ctx := componenttest.TestContext(t)
	testRunProcessorWithContext(ctx, t, processorConfig, testSignal)
}

func testRunProcessorWithContext(ctx context.Context, t *testing.T, processorConfig string, testSignal processortest.Signal) {
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.connector.spanlogs")
	require.NoError(t, err)

	var args spanlogs.Arguments
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

func Test_ComponentIO(t *testing.T) {
	const defaultInputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "TestSvcName" }
				},
				{
					"key": "res_attribute1",
					"value": { "intValue": "78" }
				},
				{
					"key": "unused_res_attribute1",
					"value": { "stringValue": "str" }
				},
				{
					"key": "res_redact_trace",
					"value": { "boolValue": true }
				},
				{
					"key": "res_account_id",
					"value": { "intValue": "2245" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
					"span_id": "086e83747d0e381e",
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "78" }
					},
					{
						"key": "unused_attribute1",
						"value": { "intValue": "78" }
					},
					{
						"key": "redact_trace",
						"value": { "boolValue": true }
					},
					{
						"key": "account_id",
						"value": { "intValue": "2245" }
					}],
					"events": [{
						"name": "log",
						"attributes": [{
							"key": "log.severity",
							"value": { "stringValue": "INFO" }
						},
						{
							"key": "log.message",
							"value": { "stringValue": "TestLogMessage" }
						}]
					},
					{
						"name": "test_event",
						"attributes": [{
							"key": "cause",
							"value": { "stringValue": "call" }
						},
						{
							"key": "ignore",
							"value": { "stringValue": "ignore" }
						}]
					}]
				}]
			}]
		}]
	}`
	defaultOverrides := spanlogs.OverrideConfig{
		LogsTag:     "traces",
		ServiceKey:  "svc",
		SpanNameKey: "span",
		StatusKey:   "status",
		DurationKey: "dur",
		TraceIDKey:  "tid",
	}

	tests := []struct {
		testName               string
		cfg                    string
		expectedUnmarshaledCfg spanlogs.Arguments
		inputTraceJson         string
		expectedOutputLogJson  string
	}{
		{
			testName: "SpansAndProcessesAndRoots",
			cfg: `
			spans = true
			roots = true
			processes = true
			labels = ["attribute1", "res_attribute1"]
			span_attributes = ["attribute1"]
			process_attributes = ["res_attribute1"]

			output {
				// no-op: will be overridden by test code.
			}
		`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Spans:             true,
				Roots:             true,
				Processes:         true,
				SpanAttributes:    []string{"attribute1"},
				ProcessAttributes: []string{"res_attribute1"},
				Overrides:         defaultOverrides,
				Labels:            []string{"attribute1", "res_attribute1"},
				Output:            &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "svc=TestSvcName res_attribute1=78 tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "process" }
							},
							{
								"key": "res_attribute1",
								"value": { "intValue": "78" }
							}]
						},
						{
							"body": { "stringValue": "span=TestSpan dur=0ns attribute1=78 svc=TestSvcName res_attribute1=78 tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "span" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "res_attribute1",
								"value": { "intValue": "78" }
							}]
						},
						{
							"body": { "stringValue": "span=TestSpan dur=0ns attribute1=78 svc=TestSvcName res_attribute1=78 tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "root" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "res_attribute1",
								"value": { "intValue": "78" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			testName: "SpansAndProcessesAndRootsWithOverrides",
			cfg: `
			spans = true
			roots = true
			processes = true
			labels = ["attribute1", "res_attribute1"]
			span_attributes = ["attribute1"]
			process_attributes = ["res_attribute1"]

			overrides {
				logs_instance_tag = "override_traces"
				service_key = "override_svc"
				span_name_key = "override_span"
				status_key = "override_status"
				duration_key = "override_dur"
				trace_id_key = "override_tid"
			}

			output {
				// no-op: will be overridden by test code.
			}
		`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Spans:             true,
				Roots:             true,
				Processes:         true,
				SpanAttributes:    []string{"attribute1"},
				ProcessAttributes: []string{"res_attribute1"},
				Overrides: spanlogs.OverrideConfig{
					LogsTag:     "override_traces",
					ServiceKey:  "override_svc",
					SpanNameKey: "override_span",
					StatusKey:   "override_status",
					DurationKey: "override_dur",
					TraceIDKey:  "override_tid",
				},
				Labels: []string{"attribute1", "res_attribute1"},
				Output: &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "override_svc=TestSvcName res_attribute1=78 override_tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "override_traces",
								"value": { "stringValue": "process" }
							},
							{
								"key": "res_attribute1",
								"value": { "intValue": "78" }
							}]
						},
						{
							"body": { "stringValue": "override_span=TestSpan override_dur=0ns attribute1=78 override_svc=TestSvcName res_attribute1=78 override_tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "override_traces",
								"value": { "stringValue": "span" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "res_attribute1",
								"value": { "intValue": "78" }
							}]
						},
						{
							"body": { "stringValue": "override_span=TestSpan override_dur=0ns attribute1=78 override_svc=TestSvcName res_attribute1=78 override_tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "override_traces",
								"value": { "stringValue": "root" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "res_attribute1",
								"value": { "intValue": "78" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			testName: "SpanAttributes",
			cfg: `
			spans = true
			labels = ["attribute1", "redact_trace", "account_id"]
			span_attributes = ["attribute1", "redact_trace", "account_id"]
	
			output {
				// no-op: will be overridden by test code.
			}
		`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Spans:          true,
				SpanAttributes: []string{"attribute1", "redact_trace", "account_id"},
				Overrides:      defaultOverrides,
				Labels:         []string{"attribute1", "redact_trace", "account_id"},
				Output:         &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "span=TestSpan dur=0ns attribute1=78 redact_trace=true account_id=2245 svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "span" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "redact_trace",
								"value": { "boolValue": true }
							},
							{
								"key": "account_id",
								"value": { "intValue": "2245" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			// Specifying an attribute in "labels" has no effect if the attribute
			// is not in "span_attributes" or "process_attributes".
			testName: "LabelNotInSpanAttributes",
			cfg: `
			spans = true
			labels = ["attribute1", "redact_trace", "account_id"]

			output {
				// no-op: will be overridden by test code.
			}
		`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Spans:     true,
				Overrides: defaultOverrides,
				Labels:    []string{"attribute1", "redact_trace", "account_id"},
				Output:    &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "span=TestSpan dur=0ns svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "span" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			testName: "ProcessAttributes",
			cfg: `
			processes = true
			labels = ["res_attribute1", "res_redact_trace", "res_account_id"]
			process_attributes = ["res_attribute1", "res_redact_trace", "res_account_id"]

			output {
				// no-op: will be overridden by test code.
			}
		`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Processes:         true,
				ProcessAttributes: []string{"res_attribute1", "res_redact_trace", "res_account_id"},
				Overrides:         defaultOverrides,
				Labels:            []string{"res_attribute1", "res_redact_trace", "res_account_id"},
				Output:            &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "svc=TestSvcName res_attribute1=78 res_redact_trace=true res_account_id=2245 tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "process" }
							},
							{
								"key": "res_attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "res_redact_trace",
								"value": { "boolValue": true }
							},
							{
								"key": "res_account_id",
								"value": { "intValue": "2245" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			// Specifying an attribute in "labels" has no effect if the attribute
			// is not in "span_attributes" or "process_attributes".
			testName: "LabelNotInProcessAttributes",
			cfg: `
			processes = true
			labels = ["res_attribute1", "res_redact_trace", "res_account_id"]

			output {
				// no-op: will be overridden by test code.
			}
		`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Processes: true,
				Overrides: defaultOverrides,
				Labels:    []string{"res_attribute1", "res_redact_trace", "res_account_id"},
				Output:    &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "process" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			testName: "RootsAttributes",
			cfg: `
			roots = true
			labels = ["attribute1", "redact_trace", "account_id"]
			span_attributes = ["attribute1", "redact_trace", "account_id"]

			output {
				// no-op: will be overridden by test code.
			}
		`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Roots:          true,
				SpanAttributes: []string{"attribute1", "redact_trace", "account_id"},
				Overrides:      defaultOverrides,
				Labels:         []string{"attribute1", "redact_trace", "account_id"},
				Output:         &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "span=TestSpan dur=0ns attribute1=78 redact_trace=true account_id=2245 svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "root" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "redact_trace",
								"value": { "boolValue": true }
							},
							{
								"key": "account_id",
								"value": { "intValue": "2245" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			testName: "RootsAttributesWhenParentSpanIsPresent",
			cfg: `
			roots = true
			labels = ["attribute1", "redact_trace", "account_id"]
			span_attributes = ["attribute1", "redact_trace", "account_id"]

			output {
				// no-op: will be overridden by test code.
			}`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Roots:          true,
				SpanAttributes: []string{"attribute1", "redact_trace", "account_id"},
				Overrides:      defaultOverrides,
				Labels:         []string{"attribute1", "redact_trace", "account_id"},
				Output:         &otelcol.ConsumerArguments{},
			},
			inputTraceJson: `{
				"resourceSpans": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						},
						{
							"key": "res_attribute1",
							"value": { "intValue": "78" }
						},
						{
							"key": "unused_res_attribute1",
							"value": { "stringValue": "str" }
						},
						{
							"key": "res_redact_trace",
							"value": { "boolValue": true }
						},
						{
							"key": "res_account_id",
							"value": { "intValue": "2245" }
						}]
					},
					"scopeSpans": [{
						"spans": [{
							"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
							"parent_span_id": "146e83747d0e381e",
							"span_id": "086e83747d0e381e",
							"name": "TestSpan",
							"attributes": [{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "unused_attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "redact_trace",
								"value": { "boolValue": true }
							},
							{
								"key": "account_id",
								"value": { "intValue": "2245" }
							}]
						}]
					}]
				}]
			}`,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": []
						}]
					}]
				}]
			}`,
		},
		{
			testName: "SpanStatusCode",
			cfg: `
			spans = true
			labels = ["attribute1", "redact_trace", "account_id"]

			output {
				// no-op: will be overridden by test code.
			}`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Spans:     true,
				Overrides: defaultOverrides,
				Labels:    []string{"attribute1", "redact_trace", "account_id"},
				Output:    &otelcol.ConsumerArguments{},
			},
			inputTraceJson: `{
				"resourceSpans": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						}]
					},
					"scopeSpans": [{
						"spans": [{
							"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
							"span_id": "086e83747d0e381e",
							"name": "TestSpan",
							"attributes": [{
								"key": "attribute1",
								"value": { "intValue": "78" }
							}],
							"status": {
								"code": 2,
								"message": "some additional error description"
							}
						}]
					}]
				}]
			}`,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "span=TestSpan dur=0ns status=Error svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "span" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			testName: "MultipleResourceSpans",
			cfg: `
			spans = true
			span_attributes = ["attribute1", "redact_trace", "account_id"]
			labels = ["attribute1", "redact_trace", "account_id"]

			output {
				// no-op: will be overridden by test code.
			}`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Spans:          true,
				SpanAttributes: []string{"attribute1", "redact_trace", "account_id"},
				Overrides:      defaultOverrides,
				Labels:         []string{"attribute1", "redact_trace", "account_id"},
				Output:         &otelcol.ConsumerArguments{},
			},
			inputTraceJson: `{
				"resourceSpans": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						}]
					},
					"scopeSpans": [{
						"spans": [{
							"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
							"span_id": "086e83747d0e381e",
							"name": "TestSpan",
							"attributes": [{
								"key": "attribute1",
								"value": { "intValue": "11111" }
							}],
							"status": {
								"code": 2,
								"message": "some additional error description"
							}
						}]
					}]
				},
				{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						}]
					},
					"scopeSpans": [{
						"spans": [{
							"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
							"span_id": "086e83747d0e381e",
							"name": "TestSpan",
							"attributes": [{
								"key": "attribute1",
								"value": { "intValue": "22222" }
							}]
						}]
					}]
				}]
			}`,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "span=TestSpan dur=0ns status=Error attribute1=11111 svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "span" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "11111" }
							}]
						}]
					}]
				},
				{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "span=TestSpan dur=0ns attribute1=22222 svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "span" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "22222" }
							}]
						}]
					}]
				}]
			}`,
		},
		{
			testName: "Events",
			cfg: `
			events = true
			span_attributes = ["attribute1", "redact_trace", "account_id"]
			event_attributes = ["log.severity", "log.message"]
			labels = ["attribute1", "redact_trace", "account_id", "log.severity", "log.message"]

			output {
				// no-op: will be overridden by test code.
			}`,
			expectedUnmarshaledCfg: spanlogs.Arguments{
				Events:          true,
				EventAttributes: []string{"log.severity", "log.message"},
				SpanAttributes:  []string{"attribute1", "redact_trace", "account_id"},
				Overrides:       defaultOverrides,
				Labels:          []string{"attribute1", "redact_trace", "account_id", "log.severity", "log.message"},
				Output:          &otelcol.ConsumerArguments{},
			},
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceLogs": [{
					"scopeLogs": [{
						"log_records": [{
							"body": { "stringValue": "span=TestSpan dur=0ns attribute1=78 redact_trace=true account_id=2245 svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d log.severity=INFO log.message=TestLogMessage" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "event" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "redact_trace", 
								"value": { "boolValue": true }
							},
							{
								"key": "account_id",
								"value": { "intValue": "2245" }
							},
							{
								"key": "log.severity",
								"value": { "stringValue": "INFO" }
							},
							{
								"key": "log.message",
								"value": { "stringValue": "TestLogMessage" }
							}]
						},
						{
							"body": { "stringValue": "span=TestSpan dur=0ns attribute1=78 redact_trace=true account_id=2245 svc=TestSvcName tid=7bba9f33312b3dbb8b2c2c62bb7abe2d" },
							"attributes": [{
								"key": "traces",
								"value": { "stringValue": "event" }
							},
							{
								"key": "attribute1",
								"value": { "intValue": "78" }
							},
							{
								"key": "redact_trace", 
								"value": { "boolValue": true }
							},
							{
								"key": "account_id",
								"value": { "intValue": "2245" }
							}]
						}]
					}]
				}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var args spanlogs.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tt.cfg), &args))
			require.EqualValues(t, tt.expectedUnmarshaledCfg, args)

			testRunProcessor(t, tt.cfg, processortest.NewTraceToLogSignal(tt.inputTraceJson, tt.expectedOutputLogJson))
		})
	}
}
