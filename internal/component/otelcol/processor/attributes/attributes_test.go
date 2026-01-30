package attributes_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/attributes"
	"github.com/grafana/alloy/internal/component/otelcol/processor/processortest"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
)

const backtick = "`"

// These are tests for SeverityLevel and not for the attributes processor as a whole.
// However, because Otel's LogSeverityNumberMatchProperties structure is internal
// we are not able ot test it directly.
// The only way is to create a whole attributesprocessor.Config, because that struct is public.
// This is why the test is in attributes_test.go instead of config_filter_test.go.
func TestSeverityLevelMatchesOtel(t *testing.T) {
	type TestDefinition struct {
		name               string
		cfg                string
		expectedOtelSevStr string
	}

	var tests []TestDefinition

	for _, testInfo := range []struct {
		alloySevStr string
		otelSevStr  string
	}{
		{"TRACE", "Trace"},
		{"TRACE2", "Trace2"},
		{"TRACE3", "Trace3"},
		{"TRACE4", "Trace4"},
		{"DEBUG", "Debug"},
		{"DEBUG2", "Debug2"},
		{"DEBUG3", "Debug3"},
		{"DEBUG4", "Debug4"},
		{"INFO", "Info"},
		{"INFO2", "Info2"},
		{"INFO3", "Info3"},
		{"INFO4", "Info4"},
		{"WARN", "Warn"},
		{"WARN2", "Warn2"},
		{"WARN3", "Warn3"},
		{"WARN4", "Warn4"},
		{"ERROR", "Error"},
		{"ERROR2", "Error2"},
		{"ERROR3", "Error3"},
		{"ERROR4", "Error4"},
		{"FATAL", "Fatal"},
		{"FATAL2", "Fatal2"},
		{"FATAL3", "Fatal3"},
		{"FATAL4", "Fatal4"},
	} {
		cfgTemplate := `
		match_type = "strict"
		log_severity {
			min = "%s"
			match_undefined = true
		}
		`

		newTest := TestDefinition{
			name:               testInfo.alloySevStr,
			cfg:                fmt.Sprintf(cfgTemplate, testInfo.alloySevStr),
			expectedOtelSevStr: testInfo.otelSevStr,
		}
		tests = append(tests, newTest)
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var matchProperties otelcol.MatchProperties
			err := syntax.Unmarshal([]byte(tt.cfg), &matchProperties)

			require.NoError(t, err)

			input := make(map[string]any)

			matchConfig, err := matchProperties.Convert()
			require.NoError(t, err)
			input["include"] = matchConfig

			var result attributesprocessor.Config
			err = mapstructure.Decode(input, &result)
			require.NoError(t, err)

			require.Equal(t, tt.expectedOtelSevStr, result.MatchConfig.Include.LogSeverityNumber.Min.String())
		})
	}
}

// A lot of the TestDecode tests were inspired by tests in the Otel repo:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.63.0/processor/attributesprocessor/testdata/config.yaml

func testRunProcessor(t *testing.T, processorConfig string, testSignal processortest.Signal) {
	ctx := componenttest.TestContext(t)
	testRunProcessorWithContext(ctx, t, processorConfig, testSignal)
}

func testRunProcessorWithContext(ctx context.Context, t *testing.T, processorConfig string, testSignal processortest.Signal) {
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.attributes")
	require.NoError(t, err)

	var args attributes.Arguments
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

// Test that the validate function is called. The validation logic for the actions is tested in the otelcol pkg.
func Test_Validate(t *testing.T) {
	cfg := `
		action {
			value = 111111
			action = "insert"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	expectedErr := "validation failed for action block number 1: the action insert requires the key argument to be set"
	var args attributes.Arguments
	require.ErrorContains(t, syntax.Unmarshal([]byte(cfg), &args), expectedErr)
}

func Test_Insert(t *testing.T) {
	cfg := `
		action {
			key = "attribute1"
			value = 111111
			action = "insert"
		}
		action {
			key = "string key"
			value = "anotherkey"
			action = "insert"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "attribute1", action.Key)
	require.Equal(t, 111111, action.Value)
	require.Equal(t, "insert", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "string key", action.Key)
	require.Equal(t, "anotherkey", action.Value)
	require.Equal(t, "insert", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
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
			"resource": {},
			"scopeSpans": [{
				"scope": {},
				"spans": [{
						"traceId": "",
						"spanId": "",
						"parentSpanId": "",
						"name": "TestSpan",
						"attributes": [{
							"key": "attribute1",
							"value": { "intValue": "0" }
						},
						{
							"key": "string key",
							"value": { "stringValue": "anotherkey" }
						}],
						"status": {}
					}]
				}]
			}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_RegexExtract(t *testing.T) {
	cfg := `
		action {
			key = "user_key"
			pattern = ` + backtick + `\/api\/v1\/document\/(?P<new_user_key>.*)\/update\/(?P<version>.*)$` + backtick + `
			action = "extract"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "user_key", action.Key)
	require.Equal(t, `\/api\/v1\/document\/(?P<new_user_key>.*)\/update\/(?P<version>.*)$`, action.RegexPattern)
	require.Equal(t, "extract", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "user_key",
						"value": { "stringValue": "/api/v1/document/12345678/update/v1" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "user_key",
						"value": { "stringValue": "/api/v1/document/12345678/update/v1" }
					},
					{
						"key": "new_user_key",
						"value": { "stringValue": "12345678" }
					},
					{
						"key": "version",
						"value": { "stringValue": "v1" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Update(t *testing.T) {
	cfg := `
		action {
			key = "boo"
			from_attribute = "foo"
			action = "update"
		}
		action {
			key = "db.secret"
			value = "redacted"
			action = "update"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "boo", action.Key)
	require.Equal(t, "foo", action.FromAttribute)
	require.Equal(t, "update", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "db.secret", action.Key)
	require.Equal(t, "redacted", action.Value)
	require.Equal(t, "update", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "foo",
						"value": { "intValue": "11111" }
					},
					{
						"key": "boo",
						"value": { "intValue": "22222" }
					},
					{
						"key": "db.secret",
						"value": { "stringValue": "top_secret" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "foo",
						"value": { "intValue": "11111" }
					},
					{
						"key": "boo",
						"value": { "intValue": "11111" }
					},
					{
						"key": "db.secret",
						"value": { "stringValue": "redacted" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Upsert(t *testing.T) {
	cfg := `
		action {
			key = "region"
			value = "planet-earth"
			action = "upsert"
		}
		action {
			key = "new_user_key"
			from_attribute = "user_key"
			action = "upsert"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "region", action.Key)
	require.Equal(t, "planet-earth", action.Value)
	require.Equal(t, "upsert", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "new_user_key", action.Key)
	require.Equal(t, "user_key", action.FromAttribute)
	require.Equal(t, "upsert", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "user_key",
						"value": { "intValue": "11111" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "user_key",
						"value": { "intValue": "11111" }
					},
					{
						"key": "region",
						"value": { "stringValue": "planet-earth" }
					},
					{
						"key": "new_user_key",
						"value": { "intValue": "11111" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Delete(t *testing.T) {
	cfg := `
		action {
			key = "credit_card"
			action = "delete"
		}
		action {
			key = "duplicate_key"
			action = "delete"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "credit_card",
						"value": { "intValue": "11111" }
					},
					{
						"key": "duplicate_key",
						"value": { "intValue": "22222" }
					},
					{
						"key": "db.secret",
						"value": { "stringValue": "top_secret" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "db.secret",
						"value": { "stringValue": "top_secret" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Hash(t *testing.T) {
	cfg := `
		action {
			key = "user.email"
			action = "hash"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "user.email", action.Key)
	require.Equal(t, "hash", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "foo",
						"value": { "intValue": "11111" }
					},
					{
						"key": "boo",
						"value": { "intValue": "22222" }
					},
					{
						"key": "user.email",
						"value": { "stringValue": "user@email.com" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "foo",
						"value": { "intValue": "11111" }
					},
					{
						"key": "boo",
						"value": { "intValue": "22222" }
					},
					{
						"key": "user.email",
						"value": { "stringValue": "0925f997eb0d742678f66d2da134d15d842d57722af5f7605c4785cb5358831b" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Convert(t *testing.T) {
	cfg := `
		action {
			key = "http.status_code"
			converted_type = "int"
			action = "convert"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "http.status_code", action.Key)
	require.Equal(t, "int", action.ConvertedType)
	require.Equal(t, "convert", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "http.status_code",
						"value": { "stringValue": "500" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan",
					"attributes": [{
						"key": "http.status_code",
						"value": { "intValue": "500" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_ExcludeMulti(t *testing.T) {
	cfg := `
	exclude {
		match_type = "strict"
		services = ["svcA", "svcB"]
		attribute {
			key = "env"
			value = "dev"
		}
		attribute {
			key = "test_request"
		}
	}
	action {
		key = "credit_card"
		action = "delete"
	}
	action {
		key = "duplicate_key"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Exclude)

	require.Equal(t, "strict", string(otelObj.Exclude.MatchType))

	svc := &otelObj.Exclude.Services[0]
	require.Equal(t, "svcA", *svc)
	svc = &otelObj.Exclude.Services[1]
	require.Equal(t, "svcB", *svc)

	attr := &otelObj.Exclude.Attributes[0]
	require.Equal(t, "env", attr.Key)
	require.Equal(t, "dev", attr.Value)

	attr = &otelObj.Exclude.Attributes[1]
	require.Equal(t, "test_request", attr.Key)

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcA" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcC" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcA" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcC" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_ExcludeResources(t *testing.T) {
	cfg := `
	exclude {
		match_type = "strict"
		resource {
			key = "host.type"
			value = "n1-standard-1"
		}
	}
	action {
		key = "credit_card"
		action = "delete"
	}
	action {
		key = "duplicate_key"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Exclude)

	res := &otelObj.Exclude.Resources[0]
	require.Equal(t, "strict", string(otelObj.Exclude.MatchType))

	require.Equal(t, "host.type", res.Key)
	require.Equal(t, "n1-standard-1", res.Value)

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_ExcludeLibrary(t *testing.T) {
	cfg := `
	exclude {
		match_type = "strict"
		library {
			name = "mongo-java-driver"
			version = "3.8.0"
		}
	}
	action {
		key = "credit_card"
		action = "delete"
	}
	action {
		key = "duplicate_key"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Exclude)
	require.Equal(t, "strict", string(otelObj.Exclude.MatchType))

	lib := &otelObj.Exclude.Libraries[0]
	require.Equal(t, "mongo-java-driver", lib.Name)
	require.Equal(t, "3.8.0", *lib.Version)

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": "3.8.0"
				},
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "dummy-driver",
					"version": "1.1.0"
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": "3.8.0"
				},
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "dummy-driver",
					"version": "1.1.0"
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_ExcludeLibraryAnyVersion(t *testing.T) {
	cfg := `
	exclude {
		match_type = "strict"
		library {
			name = "mongo-java-driver"
		}
	}
	action {
		key = "credit_card"
		action = "delete"
	}
	action {
		key = "duplicate_key"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Exclude)
	require.Equal(t, "strict", string(otelObj.Exclude.MatchType))

	lib := &otelObj.Exclude.Libraries[0]
	require.Equal(t, "mongo-java-driver", lib.Name)
	require.Nil(t, lib.Version)

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": "3.8.0"
				},
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "dummy-driver",
					"version": "1.1.0"
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": "3.8.0"
				},
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "dummy-driver",
					"version": "1.1.0"
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_ExcludeLibraryBlankVersion(t *testing.T) {
	cfg := `
	exclude {
		match_type = "strict"
		library {
			name = "mongo-java-driver"
			version = ""
		}
	}
	action {
		key = "credit_card"
		action = "delete"
	}
	action {
		key = "duplicate_key"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Exclude)
	require.Equal(t, "strict", string(otelObj.Exclude.MatchType))

	lib := &otelObj.Exclude.Libraries[0]
	require.Equal(t, "mongo-java-driver", lib.Name)
	require.Equal(t, "", *lib.Version)

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": "3.8.0"
				},
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": ""
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "dummy-driver",
					"version": "1.1.0"
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "host.type",
					"value": { "stringValue": "n1-standard-1" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": "3.8.0"
				},
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "mongo-java-driver",
					"version": ""
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcB" }
				}]
			},
			"scopeSpans": [{
				"scope": {
					"name": "dummy-driver",
					"version": "1.1.0"
				},
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_ExcludeServices(t *testing.T) {
	cfg := `
	exclude {
		match_type = "regexp"
		services = ["auth.*", "login.*"]
	}
	action {
		key = "credit_card"
		action = "delete"
	}
	action {
		key = "duplicate_key"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Exclude)
	require.Equal(t, "regexp", string(otelObj.Exclude.MatchType))

	svc := &otelObj.Exclude.Services
	require.Equal(t, "auth.*", (*svc)[0])
	require.Equal(t, "login.*", (*svc)[1])

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "auth.basic" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "login.user" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcC" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "auth.basic" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "login.user" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcC" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "test_request",
						"value": { "stringValue": "req_body" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_SelectiveProcessing(t *testing.T) {
	cfg := `
	include {
		match_type = "strict"
		services = ["svcA", "svcB"]
	}
	exclude {
		match_type = "strict"
		attribute {
			key = "redact_trace"
			value = false
		}
	}
	action {
		key = "credit_card"
		action = "delete"
	}
	action {
		key = "duplicate_key"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Include)
	require.Equal(t, "strict", string(otelObj.Include.MatchType))

	svc := &otelObj.Include.Services
	require.Equal(t, "svcA", (*svc)[0])
	require.Equal(t, "svcB", (*svc)[1])

	require.NotNil(t, otelObj.Exclude)
	require.Equal(t, "strict", string(otelObj.Exclude.MatchType))

	attr := &otelObj.Exclude.Attributes[0]
	require.Equal(t, "redact_trace", attr.Key)
	require.Equal(t, false, attr.Value)

	action := &otelObj.Actions[0]
	require.Equal(t, "credit_card", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "duplicate_key", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcA" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "redact_trace",
						"value": { "boolValue": true }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "login.user" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "redact_trace",
						"value": { "boolValue": true }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcA" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "redact_trace",
						"value": { "boolValue": true }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "login.user" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "redact_trace",
						"value": { "boolValue": true }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Complex(t *testing.T) {
	cfg := `
	action {
		key = "operation"
		value = "default"
		action = "insert"
	}
	action {
		key = "svc.operation"
		value = "operation"
		action = "upsert"
	}
	action {
		key = "operation"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "operation", action.Key)
	require.Equal(t, "default", action.Value)
	require.Equal(t, "insert", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "svc.operation", action.Key)
	require.Equal(t, "operation", action.Value)
	require.Equal(t, "upsert", string(action.Action))

	action = &otelObj.Actions[2]
	require.Equal(t, "operation", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcA" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "svc.operation",
						"value": { "stringValue": "old_operation" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "svcA" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "env",
						"value": { "stringValue": "dev" }
					},
					{
						"key": "svc.operation",
						"value": { "stringValue": "operation" }
					},
					{
						"key": "credit_card",
						"value": { "stringValue": "0000-00000-00000" }
					},
					{
						"key": "duplicate_key",
						"value": { "stringValue": "deuplicateduplicatekey" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_ExampleActions(t *testing.T) {
	cfg := `
	action {
		key = "db.table"
		action = "delete"
	}
	action {
		key = "redacted_span"
		value = true
		action = "upsert"
	}
	action {
		key = "copy_key"
		from_attribute = "key_original"
		action = "update"
	}
	action {
		key = "account_id"
		value = 2245
		action = "insert"
	}
	action {
		key = "account_password"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "db.table", action.Key)
	require.Equal(t, "delete", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "redacted_span", action.Key)
	require.Equal(t, true, action.Value)
	require.Equal(t, "upsert", string(action.Action))

	action = &otelObj.Actions[2]
	require.Equal(t, "copy_key", action.Key)
	require.Equal(t, "key_original", action.FromAttribute)
	require.Equal(t, "update", string(action.Action))

	action = &otelObj.Actions[3]
	require.Equal(t, "account_id", action.Key)
	require.Equal(t, 2245, otelObj.Actions[3].Value)
	require.Equal(t, "insert", string(action.Action))

	action = &otelObj.Actions[4]
	require.Equal(t, "account_password", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "db.table",
						"value": { "stringValue": "users" }
					},
					{
						"key": "key_original",
						"value": { "stringValue": "original_data" }
					},
					{
						"key": "copy_key",
						"value": { "stringValue": "non_original_data" }
					},
					{
						"key": "account_password",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "account_id",
						"value": { "intValue": "2245" }
					},
					{
						"key": "key_original",
						"value": { "stringValue": "original_data" }
					},
					{
						"key": "copy_key",
						"value": { "stringValue": "original_data" }
					},
					{
						"key": "redacted_span",
						"value": { "boolValue": true }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Regexp(t *testing.T) {
	cfg := `
	include {
		match_type = "regexp"
		services = ["auth.*"]
	}
	exclude {
		match_type = "regexp"
		span_names = ["login.*"]
	}
	action {
		key = "password"
		action = "update"
		value = "obfuscated"
	}
	action {
		key = "token"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Include)
	require.Equal(t, "regexp", string(otelObj.Include.MatchType))
	require.Equal(t, "auth.*", otelObj.Include.Services[0])

	require.NotNil(t, otelObj.Exclude)
	require.Equal(t, "regexp", string(otelObj.Exclude.MatchType))
	require.Equal(t, "login.*", otelObj.Exclude.SpanNames[0])

	action := &otelObj.Actions[0]
	require.Equal(t, "password", action.Key)
	require.Equal(t, "obfuscated", action.Value)
	require.Equal(t, "update", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "token", action.Key)
	require.Equal(t, "delete", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "auth.basic" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "secret_token" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "login.user" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "secret_token" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "auth.basic" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "obfuscated" }
					}]
				}]
			}]
		},
		{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "login.user" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "secret_token" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_Regexp2(t *testing.T) {
	cfg := `
	include {
		match_type = "regexp"
		attribute {
			key = "db.statement"
			value = "SELECT \\* FROM USERS.*"
		}
	}
	action {
		key = "db.statement"
		action = "update"
		value = "SELECT * FROM USERS [obfuscated]"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Include)
	require.Equal(t, "regexp", string(otelObj.Include.MatchType))

	attr := &otelObj.Include.Attributes[0]
	require.Equal(t, "db.statement", attr.Key)
	require.Equal(t, "SELECT \\* FROM USERS.*", attr.Value)

	action := &otelObj.Actions[0]
	require.Equal(t, "db.statement", action.Key)
	require.Equal(t, "SELECT * FROM USERS [obfuscated]", action.Value)
	require.Equal(t, "update", string(action.Action))

	var inputTrace = `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "db.statement",
						"value": { "stringValue": "SELECT * FROM USERS_SECRETS" }
					}]
				}]
			}]
		},
		{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "db.statement",
						"value": { "stringValue": "SELECT * FROM PRODUCTS" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputTrace := `{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan1",
					"attributes": [{
						"key": "db.statement",
						"value": { "stringValue": "SELECT * FROM USERS [obfuscated]" }
					}]
				}]
			}]
		},
		{
			"scopeSpans": [{
				"spans": [{
					"name": "TestSpan2",
					"attributes": [{
						"key": "db.statement",
						"value": { "stringValue": "SELECT * FROM PRODUCTS" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewTraceSignal(inputTrace, expectedOutputTrace))
}

func Test_LogBodyRegexp(t *testing.T) {
	cfg := `
	include {
		match_type = "regexp"
		log_bodies = ["AUTH.*"]
	}
	action {
		key = "password"
		action = "update"
		value = "obfuscated"
	}
	action {
		key = "token"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Include)
	require.Equal(t, "regexp", string(otelObj.Include.MatchType))

	require.Equal(t, "AUTH.*", otelObj.Include.LogBodies[0])

	action := &otelObj.Actions[0]
	require.Equal(t, "password", action.Key)
	require.Equal(t, "obfuscated", action.Value)
	require.Equal(t, "update", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "token", otelObj.Actions[1].Key)
	require.Equal(t, "delete", string(action.Action))

	var inputLog = `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "Info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "fake_token" }
					}]
				}]
			}]
		},
		{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000999",
					"severityNumber": 9,
					"severityText": "Info",
					"name": "logA",
					"body": { "stringValue": "This is a log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "fake_token" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputLog := `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "Info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "obfuscated" }
					}]
				}]
			}]
		},
		{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000999",
					"severityNumber": 9,
					"severityText": "Info",
					"name": "logA",
					"body": { "stringValue": "This is a log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "fake_token" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewLogSignal(inputLog, expectedOutputLog))
}

func Test_LogSeverityTextsRegexp(t *testing.T) {
	cfg := `
	include {
		match_type = "regexp"
		log_severity_texts = ["info.*"]
	}
	action {
		key = "password"
		action = "update"
		value = "obfuscated"
	}
	action {
		key = "token"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Include)
	require.Equal(t, "regexp", string(otelObj.Include.MatchType))

	require.Equal(t, "info.*", otelObj.Include.LogSeverityTexts[0])

	action := &otelObj.Actions[0]
	require.Equal(t, "password", action.Key)
	require.Equal(t, "obfuscated", action.Value)
	require.Equal(t, "update", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "token", otelObj.Actions[1].Key)
	require.Equal(t, "delete", string(action.Action))

	var inputLog = `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		},
		{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000999",
					"severityNumber": 5,
					"severityText": "debug",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputLog := `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "obfuscated" }
					}]
				}]
			}]
		},
		{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000999",
					"severityNumber": 5,
					"severityText": "debug",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewLogSignal(inputLog, expectedOutputLog))
}

func Test_LogSeverity(t *testing.T) {
	cfg := `
	include {
		match_type = "regexp"
		log_severity {
			min = "INFO"
			match_undefined = true
		}
	}
	action {
		key = "password"
		action = "update"
		value = "obfuscated"
	}
	action {
		key = "token"
		action = "delete"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Include)
	require.Equal(t, "regexp", string(otelObj.Include.MatchType))

	require.Equal(t, int32(9), int32(otelObj.Include.LogSeverityNumber.Min))
	require.Equal(t, true, otelObj.Include.LogSeverityNumber.MatchUndefined)

	action := &otelObj.Actions[0]
	require.Equal(t, "password", action.Key)
	require.Equal(t, "obfuscated", action.Value)
	require.Equal(t, "update", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "token", otelObj.Actions[1].Key)
	require.Equal(t, "delete", string(action.Action))

	var inputLog = `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		},
		{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000999",
					"severityNumber": 5,
					"severityText": "debug",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputLog := `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "obfuscated" }
					}]
				}]
			}]
		},
		{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000999",
					"severityNumber": 5,
					"severityText": "debug",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "token",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewLogSignal(inputLog, expectedOutputLog))
}

func Test_FromContext(t *testing.T) {
	cfg := `
	action {
		key = "origin"
		from_context = "metadata.origin"
		action = "insert"
	}
	action {
		key = "enduser.id"
		from_context = "auth.subject"
		action = "insert"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)

	otelObj := (convertedArgs).(*attributesprocessor.Config)

	action := &otelObj.Actions[0]
	require.Equal(t, "origin", action.Key)
	require.Equal(t, "metadata.origin", action.FromContext)
	require.Equal(t, "insert", string(action.Action))

	action = &otelObj.Actions[1]
	require.Equal(t, "enduser.id", action.Key)
	require.Equal(t, "auth.subject", action.FromContext)
	require.Equal(t, "insert", string(action.Action))

	var inputLog = `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					}]
				}]
			}]
		}]
	}`

	expectedOutputLog := `{
		"resourceLogs": [{
			"scopeLogs": [{
				"log_records": [{
					"timeUnixNano": "1581452773000000111",
					"severityNumber": 9,
					"severityText": "info",
					"name": "logA",
					"body": { "stringValue": "AUTH log message" },
					"attributes": [{
						"key": "password",
						"value": { "stringValue": "12345" }
					},
					{
						"key": "origin",
						"value": { "stringValue": "fake_origin" }
					},
					{
						"key": "enduser.id",
						"value": { "stringValue": "fake_subject" }
					}]
				}]
			}]
		}]
	}`

	ctx := componenttest.TestContext(t)
	ctx = client.NewContext(ctx, client.Info{
		Addr: &net.IPAddr{
			IP: net.ParseIP(net.ParseIP("0.0.0.0").String()),
		},
		Auth:     fakeAuthData{},
		Metadata: client.NewMetadata(map[string][]string{"origin": {"fake_origin"}}),
	})
	testRunProcessorWithContext(ctx, t, cfg, processortest.NewLogSignal(inputLog, expectedOutputLog))
}

var _ client.AuthData = (*fakeAuthData)(nil)

type fakeAuthData struct{}

func (fakeAuthData) GetAttribute(name string) any {
	return "fake_subject"
}

func (fakeAuthData) GetAttributeNames() []string {
	return []string{"subject"}
}

func Test_MetricNames(t *testing.T) {
	cfg := `
	include {
		match_type = "regexp"
		metric_names = ["counter.*"]
	}
	action {
		key = "important_label"
		action = "upsert"
		value = "label_val"
	}

	output {
		// no-op: will be overridden by test code.
	}
	`
	var args attributes.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	convertedArgs, err := args.Convert()
	require.NoError(t, err)
	otelObj := (convertedArgs).(*attributesprocessor.Config)

	require.NotNil(t, otelObj.Include)
	require.Equal(t, "regexp", string(otelObj.Include.MatchType))

	require.Equal(t, "counter.*", otelObj.Include.MetricNames[0])

	action := &otelObj.Actions[0]
	require.Equal(t, "important_label", action.Key)
	require.Equal(t, "label_val", action.Value)
	require.Equal(t, "upsert", string(action.Action))

	var inputMetric = `{
		"resourceMetrics": [{
			"scopeMetrics": [{
				"metrics": [{
					"name": "counter-int",
					"unit": "1",
					"sum": {
						"dataPoints": [{
							"attributes": [{
								"key": "label-1",
								"value": { "stringValue": "label-value-1" }
							},
							{
								"key": "label2",
								"value": { "stringValue": "label-value-2" }
							}],
							"startTimeUnixNano": "1581452773000000789",
							"timeUnixNano": "1581452773000000789",
							"asInt": "123"
						}],
						"aggregationTemporality": 2,
						"isMonotonic": true
					}
				}]
			}]
		},
		{
			"scopeMetrics": [{
				"metrics": [{
					"name": "c-int",
					"unit": "1",
					"sum": {
						"dataPoints": [{
							"attributes": [{
								"key": "label-1",
								"value": { "stringValue": "label-value-1" }
							},
							{
								"key": "label2",
								"value": { "stringValue": "label-value-2" }
							}],
							"startTimeUnixNano": "1581452772000000321",
							"timeUnixNano": "1581452773000000789",
							"asInt": "456"
						}],
						"aggregationTemporality": 2,
						"isMonotonic": true
					}
				}]
			}]
		}]
	}`

	expectedOutputMetric := `{
		"resourceMetrics": [{
			"scopeMetrics": [{
				"metrics": [{
					"name": "counter-int",
					"unit": "1",
					"sum": {
						"dataPoints": [{
							"attributes": [{
								"key": "label-1",
								"value": { "stringValue": "label-value-1" }
							},
							{
								"key": "label2",
								"value": { "stringValue": "label-value-2" }
							},
							{
								"key": "important_label",
								"value": { "stringValue": "label_val" }
							}],
							"startTimeUnixNano": "1581452773000000789",
							"timeUnixNano": "1581452773000000789",
							"asInt": "123"
						}],
						"aggregationTemporality": 2,
						"isMonotonic": true
					}
				}]
			}]
		},
		{
			"scopeMetrics": [{
				"metrics": [{
					"name": "c-int",
					"unit": "1",
					"sum": {
						"dataPoints": [{
							"attributes": [{
								"key": "label-1",
								"value": { "stringValue": "label-value-1" }
							},
							{
								"key": "label2",
								"value": { "stringValue": "label-value-2" }
							}],
							"startTimeUnixNano": "1581452772000000321",
							"timeUnixNano": "1581452773000000789",
							"asInt": "456"
						}],
						"aggregationTemporality": 2,
						"isMonotonic": true
					}
				}]
			}]
		}]
	}`

	testRunProcessor(t, cfg, processortest.NewMetricSignal(inputMetric, expectedOutputMetric))
}
