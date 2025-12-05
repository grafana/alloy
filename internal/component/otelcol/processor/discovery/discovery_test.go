package discovery_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/processor/discovery"
	"github.com/grafana/alloy/internal/component/otelcol/processor/processortest"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	promsdconsumer "github.com/grafana/alloy/internal/static/traces/promsdprocessor/consumer"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.5.0"
)

func testRunProcessor(t *testing.T, processorConfig string, testSignal processortest.Signal) {
	ctx := componenttest.TestContext(t)
	testRunProcessorWithContext(ctx, t, processorConfig, testSignal)
}

func testRunProcessorWithContext(ctx context.Context, t *testing.T, processorConfig string, testSignal processortest.Signal) {
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.discovery")
	require.NoError(t, err)

	var args discovery.Arguments
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

func Test_DefaultConfig(t *testing.T) {
	cfg := `
		targets = [{
			"__address__" = "1.2.2.2",
			"__internal_label__" = "test_val",
			"test_label" = "test_val2"}]

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args discovery.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	var defaultArgs discovery.Arguments
	defaultArgs.SetToDefault()
	require.Equal(t, args.OperationType, promsdconsumer.OperationTypeUpsert)
	require.Equal(t, args.PodAssociations, defaultArgs.PodAssociations)

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.3.3.3" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				},{
					"key": "test_label",
					"value": { "stringValue": "test_val2" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.3.3.3" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Insert(t *testing.T) {
	cfg := `
		targets = [{
			"__address__" = "1.2.2.2",
			"__internal_label__" = "test_val",
			"test_label" = "test_val2",
			"test.label.with.dots" = "test.val2.with.dots"}]

		operation_type = "insert"

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args discovery.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	var defaultArgs discovery.Arguments
	defaultArgs.SetToDefault()
	require.Equal(t, args.OperationType, promsdconsumer.OperationTypeInsert)
	require.Equal(t, args.PodAssociations, defaultArgs.PodAssociations)

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				},
				{
					"key": "test_label",
					"value": { "stringValue": "old_val" }
				},
				{
					"key": "test.label.with.dots",
					"value": { "stringValue": "old_val" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				},
				{
					"key": "test_label",
					"value": { "stringValue": "old_val" }
				},
				{
					"key": "test.label.with.dots",
					"value": { "stringValue": "old_val" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				},
				{
					"key": "test.label.with.dots",
					"value": { "stringValue": "test.val2.with.dots" }
				},
				{
					"key": "test_label",
					"value": { "stringValue": "test_val2" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Update(t *testing.T) {
	cfg := `
		targets = [{
			"__address__" = "1.2.2.2",
			"__internal_label__" = "test_val",
			"test_label" = "test_val2",
			"test.label.with.dots" = "test.val2.with.dots"}]

		operation_type = "update"

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args discovery.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	var defaultArgs discovery.Arguments
	defaultArgs.SetToDefault()
	require.Equal(t, args.OperationType, promsdconsumer.OperationTypeUpdate)
	require.Equal(t, args.PodAssociations, defaultArgs.PodAssociations)

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				},
				{
					"key": "test_label",
					"value": { "stringValue": "old_val" }
				},
				{
					"key": "test.label.with.dots",
					"value": { "stringValue": "old_val" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				},
				{
					"key": "test_label",
					"value": { "stringValue": "test_val2" }
				},
				{
					"key": "test.label.with.dots",
					"value": { "stringValue": "test.val2.with.dots" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "ip",
					"value": { "stringValue": "1.2.2.2" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "0" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_PodAssociationLabels(t *testing.T) {
	for _, podAssociationLabel := range []string{
		promsdconsumer.PodAssociationIPLabel,
		promsdconsumer.PodAssociationOTelIPLabel,
		promsdconsumer.PodAssociationk8sIPLabel,
		promsdconsumer.PodAssociationHostnameLabel,
		//TODO: Write a test for PodAssociationConnectionIP
		// promsdconsumer.PodAssociationConnectionIP,
	} {
		cfg := fmt.Sprintf(`
		targets = [{
			"__address__" = "1.2.2.2",
			"__internal_label__" = "test_val",
			"test_label" = "test_val2"}]

		operation_type = "insert"
		pod_associations = ["%s"]

		output {
			// no-op: will be overridden by test code.
		}
		`, podAssociationLabel)

		var args discovery.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

		require.Equal(t, args.OperationType, promsdconsumer.OperationTypeInsert)
		require.Equal(t, args.PodAssociations, []string{podAssociationLabel})

		resourceLabel := podAssociationLabel
		if resourceLabel == promsdconsumer.PodAssociationHostnameLabel {
			resourceLabel = string(semconv.HostNameKey)
		}
		var inputTrace = fmt.Sprintf(`{
			"resourceSpans": [{
				"resource": {
					"attributes": [{
						"key": "%s",
						"value": { "stringValue": "1.2.2.2" }
					}]
				},
				"scopeSpans": [{
					"spans": [{
						"name": "TestSpan",
						"attributes": [{
							"key": "attribute1",
							"value": { "intValue": "0" }
						}]
					}]
				}]
			}]
		}`, resourceLabel)

		expectedOutputTrace := fmt.Sprintf(`{
			"resourceSpans": [{
				"resource": {
					"attributes": [{
						"key": "%s",
						"value": { "stringValue": "1.2.2.2" }
					},
					{
						"key": "test_label",
						"value": { "stringValue": "test_val2" }
					}]
				},
				"scopeSpans": [{
					"spans": [{
						"name": "TestSpan",
						"attributes": [{
							"key": "attribute1",
							"value": { "intValue": "0" }
						}]
					}]
				}]
			}]
		}`, resourceLabel)

		testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
	}
}
