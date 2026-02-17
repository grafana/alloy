package transform_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/internal/testutils"
	"github.com/grafana/alloy/internal/component/otelcol/processor/transform"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/require"
)

const backtick = "`"

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]any
		errorMsg string
	}{
		{
			testName: "Defaults",
			cfg: `
			output {}
			`,
			expected: map[string]any{
				"error_mode":         "propagate",
				"trace_statements":   []any{},
				"metric_statements":  []any{},
				"log_statements":     []any{},
				"profile_statements": []any{},
			},
		},
		{
			testName: "IgnoreErrors",
			cfg: `
			error_mode = "ignore"
			output {}
			`,
			expected: map[string]any{
				"error_mode":         "ignore",
				"trace_statements":   []any{},
				"metric_statements":  []any{},
				"log_statements":     []any{},
				"profile_statements": []any{},
			},
		},
		{
			testName: "TransformIfFieldDoesNotExist",
			cfg: `
			error_mode = "ignore"
			trace_statements {
				context = "span"
				statements = [
					// Accessing a map with a key that does not exist will return nil.
					` + backtick + `set(attributes["test"], "pass") where attributes["test"] == nil` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "span",
						"statements": []any{
							`set(attributes["test"], "pass") where attributes["test"] == nil`,
						},
					},
				},
				"metric_statements":  []any{},
				"log_statements":     []any{},
				"profile_statements": []any{},
			},
		},
		{
			testName: "TransformWithConditions",
			cfg: `
			error_mode = "ignore"
			trace_statements {
				context = "span"
				statements = [
					` + backtick + `set(name, "bear")` + backtick + `,
				]
				conditions = [
					` + backtick + `attributes["http.path"] == "/animal"` + backtick + `,
				]
			}
			metric_statements {
				context = "datapoint"
				statements = [
					` + backtick + `set(metric.name, "bear")` + backtick + `,
				]
				conditions = [
					` + backtick + `attributes["http.path"] == "/animal"` + backtick + `,
				]
			}
			log_statements {
				context = "log"
				statements = [
					` + backtick + `set(body, "bear")` + backtick + `,
				]
				conditions = [
					` + backtick + `attributes["http.path"] == "/animal"` + backtick + `,
				]
			}

			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "span",
						"statements": []any{
							`set(name, "bear")`,
						},
						"conditions": []any{
							`attributes["http.path"] == "/animal"`,
						},
					},
				},
				"metric_statements": []any{
					map[string]any{
						"context": "datapoint",
						"statements": []any{
							`set(metric.name, "bear")`,
						},
						"conditions": []any{
							`attributes["http.path"] == "/animal"`,
						},
					},
				},
				"log_statements": []any{
					map[string]any{
						"context": "log",
						"statements": []any{
							`set(body, "bear")`,
						},
						"conditions": []any{
							`attributes["http.path"] == "/animal"`,
						},
					},
				},
				"profile_statements": []any{},
			},
		},
		{
			testName: "TransformWithContextStatementsErrorMode",
			cfg: `
			error_mode = "ignore"
			trace_statements {
				error_mode = "propagate"
				context = "resource"
				statements = [
					` + backtick + `set(resource.attributes["name"], "propagate")` + backtick + `,
				]
			}
			trace_statements {
				context = "resource"
				statements = [
					` + backtick + `set(resource.attributes["name"], "ignore")` + backtick + `,
				]
			}
			metric_statements {
				context = "resource"
				error_mode = "silent"
				statements = [
					` + backtick + `set(resource.attributes["name"], "silent")` + backtick + `,
				]
			}
			metric_statements {
				context = "resource"
				statements = [
					` + backtick + `set(resource.attributes["name"], "ignore")` + backtick + `,
				]
			}
			log_statements {
				context = "resource"
				error_mode = "propagate"
				statements = [
					` + backtick + `set(resource.attributes["name"], "propagate")` + backtick + `,
				]
			}
			log_statements {
				context = "resource"
				statements = [
					` + backtick + `set(resource.attributes["name"], "ignore")` + backtick + `,
				]
			}

			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"error_mode": "propagate",
						"context":    "resource",
						"statements": []any{
							`set(resource.attributes["name"], "propagate")`,
						},
					},
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(resource.attributes["name"], "ignore")`,
						},
					},
				},
				"metric_statements": []any{
					map[string]any{
						"error_mode": "silent",
						"context":    "resource",
						"statements": []any{
							`set(resource.attributes["name"], "silent")`,
						},
					},
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(resource.attributes["name"], "ignore")`,
						},
					},
				},
				"log_statements": []any{
					map[string]any{
						"error_mode": "propagate",
						"context":    "resource",
						"statements": []any{
							`set(resource.attributes["name"], "propagate")`,
						},
					},
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(resource.attributes["name"], "ignore")`,
						},
					},
				},
				"profile_statements": []any{},
			},
		},
		{
			testName: "RenameAttribute1",
			cfg: `
			error_mode = "ignore"
			trace_statements {
				context = "resource"
				statements = [
					` + backtick + `set(attributes["namespace"], attributes["k8s.namespace.name"])` + backtick + `,
					` + backtick + `delete_key(attributes, "k8s.namespace.name")` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(attributes["namespace"], attributes["k8s.namespace.name"])`,
							`delete_key(attributes, "k8s.namespace.name")`,
						},
					},
				},
				"metric_statements":  []any{},
				"log_statements":     []any{},
				"profile_statements": []any{},
			},
		},
		{
			testName: "FlatConfiguration",
			cfg: `
			error_mode = "ignore"
			statements {
				trace = [
					` + backtick + `set(span.name, "bear") where span.attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `set(resource.attributes["name"], "bear")` + backtick + `,
				]
				metric = [
					` + backtick + `set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `set(resource.attributes["name"], "bear")` + backtick + `,
				]
				log = [
					` + backtick + `set(log.body, "bear") where log.attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `set(resource.attributes["name"], "bear")` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "",
						"statements": []any{
							`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				"metric_statements": []any{
					map[string]any{
						"context": "",
						"statements": []any{
							`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				"log_statements": []any{
					map[string]any{
						"context": "",
						"statements": []any{
							`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				"profile_statements": []any{},
			},
		},
		{
			testName: "MixedConfigurationStyles",
			cfg: `
			error_mode = "ignore"
			statements {
				trace = [
					` + backtick + `set(span.name, "bear") where span.attributes["http.path"] == "/animal"` + backtick + `,
				]
				metric = [
					` + backtick + `set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"` + backtick + `,
				]
				log = [
					` + backtick + `set(log.body, "bear") where log.attributes["http.path"] == "/animal"` + backtick + `,
				]
			}
			trace_statements {
				context = "span"
				statements = [
					` + backtick + `set(attributes["name"], "bear")` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			metric_statements {
				context = "resource"
				statements = [
					` + backtick + `set(attributes["name"], "bear")` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			log_statements {
				context = "resource"
				statements = [
					` + backtick + `set(attributes["name"], "bear")` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "span",
						"statements": []any{
							`set(attributes["name"], "bear")`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					map[string]any{
						"context": "",
						"statements": []any{
							`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`,
						},
					},
				},
				"metric_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(attributes["name"], "bear")`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					map[string]any{
						"context": "",
						"statements": []any{
							`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`,
						},
					},
				},
				"log_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(attributes["name"], "bear")`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					map[string]any{
						"context": "",
						"statements": []any{
							`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`,
						},
					},
				},
				"profile_statements": []any{},
			},
		},
		{
			testName: "InvalidFlatConfiguration",
			cfg: `
			error_mode = "ignore"
			statements {
				trace = [
					` + backtick + `set(span.name, "bear") where span.attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `set(resource.attributes["name"], "bear")` + backtick + `,
					` + backtick + `set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `inferred context "metric" is not a valid candidate`,
		},
		{
			testName: "RenameAttribute2",
			cfg: `
			error_mode = "ignore"
			trace_statements {
				context = "resource"
				statements = [
					` + backtick + `replace_all_patterns(attributes, "key", "k8s\\.namespace\\.name", "namespace")` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`replace_all_patterns(attributes, "key", "k8s\\.namespace\\.name", "namespace")`,
						},
					},
				},
				"metric_statements":  []any{},
				"log_statements":     []any{},
				"profile_statements": []any{},
			},
		},
		{
			testName: "CreateAttributeFromContentOfLogBody",
			cfg: `
			error_mode = "ignore"
			log_statements {
				context = "log"
				statements = [
					` + backtick + `set(attributes["body"], body)` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"log_statements": []any{
					map[string]any{
						"context": "log",
						"statements": []any{
							`set(attributes["body"], body)`,
						},
					},
				},
				"metric_statements":  []any{},
				"trace_statements":   []any{},
				"profile_statements": []any{},
			},
		},
		{
			testName: "CombineTwoAttributes",
			cfg: `
			error_mode = "ignore"
			trace_statements {
				context = "resource"
				statements = [
					// The Concat function combines any number of strings, separated by a delimiter.
					` + backtick + `set(attributes["test"], Concat([attributes["foo"], attributes["bar"]], " "))` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(attributes["test"], Concat([attributes["foo"], attributes["bar"]], " "))`,
						},
					},
				},
				"metric_statements":  []any{},
				"log_statements":     []any{},
				"profile_statements": []any{},
			},
		},
		{
			testName: "ParseJsonLogs",
			cfg: `
			error_mode = "ignore"
			log_statements {
				context = "log"
				statements = [
					` + backtick + `merge_maps(cache, ParseJSON(body), "upsert") where IsMatch(body, "^\\{")` + backtick + `,
					` + backtick + `set(attributes["attr1"], cache["attr1"])` + backtick + `,
					` + backtick + `set(attributes["attr2"], cache["attr2"])` + backtick + `,
					` + backtick + `set(attributes["nested.attr3"], cache["nested"]["attr3"])` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"log_statements": []any{
					map[string]any{
						"context": "log",
						"statements": []any{
							`merge_maps(cache, ParseJSON(body), "upsert") where IsMatch(body, "^\\{")`,
							`set(attributes["attr1"], cache["attr1"])`,
							`set(attributes["attr2"], cache["attr2"])`,
							`set(attributes["nested.attr3"], cache["nested"]["attr3"])`,
						},
					},
				},
				"metric_statements":  []any{},
				"profile_statements": []any{},
				"trace_statements":   []any{},
			},
		},
		{
			testName: "ManyStatements1",
			cfg: `
			error_mode = "ignore"
			trace_statements {
				context = "resource"
				statements = [
					` + backtick + `keep_keys(attributes, ["service.name", "service.namespace", "cloud.region", "process.command_line"])` + backtick + `,
					` + backtick + `replace_pattern(attributes["process.command_line"], "password\\=[^\\s]*(\\s?)", "password=***")` + backtick + `,
					` + backtick + `limit(attributes, 100, [])` + backtick + `,
					` + backtick + `truncate_all(attributes, 4096)` + backtick + `,
				]
			}
			trace_statements {
				context = "span"
				statements = [
					` + backtick + `set(status.code, 1) where attributes["http.path"] == "/health"` + backtick + `,
					` + backtick + `set(name, attributes["http.route"])` + backtick + `,
					` + backtick + `replace_match(attributes["http.target"], "/user/*/list/*", "/user/{userId}/list/{listId}")` + backtick + `,
					` + backtick + `limit(attributes, 100, [])` + backtick + `,
					` + backtick + `truncate_all(attributes, 4096)` + backtick + `,
				]
			}
			metric_statements {
				context = "resource"
				statements = [
					` + backtick + `keep_keys(attributes, ["host.name"])` + backtick + `,
					` + backtick + `truncate_all(attributes, 4096)` + backtick + `,
				]
			}
			metric_statements {
				context = "metric"
				statements = [
					` + backtick + `set(description, "Sum") where type == "Sum"` + backtick + `,
					` + backtick + `convert_sum_to_gauge() where name == "system.processes.count"` + backtick + `,
					` + backtick + `convert_gauge_to_sum("cumulative", false) where name == "prometheus_metric"` + backtick + `,
					` + backtick + `aggregate_on_attributes("sum") where name == "system.memory.usage"` + backtick + `,
				]
			}
			metric_statements {
				context = "datapoint"
				statements = [
					` + backtick + `limit(attributes, 100, ["host.name"])` + backtick + `,
					` + backtick + `truncate_all(attributes, 4096)` + backtick + `,
				]
			}
			log_statements {
				context = "resource"
				statements = [
					` + backtick + `keep_keys(attributes, ["service.name", "service.namespace", "cloud.region"])` + backtick + `,
				]
			}
			log_statements {
				context = "log"
				statements = [
					` + backtick + `set(severity_text, "FAIL") where body == "request failed"` + backtick + `,
					` + backtick + `replace_all_matches(attributes, "/user/*/list/*", "/user/{userId}/list/{listId}")` + backtick + `,
					` + backtick + `replace_all_patterns(attributes, "value", "/account/\\d{4}", "/account/{accountId}")` + backtick + `,
					` + backtick + `set(body, attributes["http.route"])` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "ignore",
				"trace_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`keep_keys(attributes, ["service.name", "service.namespace", "cloud.region", "process.command_line"])`,
							`replace_pattern(attributes["process.command_line"], "password\\=[^\\s]*(\\s?)", "password=***")`,
							`limit(attributes, 100, [])`,
							`truncate_all(attributes, 4096)`,
						},
					},
					map[string]any{
						"context": "span",
						"statements": []any{
							`set(status.code, 1) where attributes["http.path"] == "/health"`,
							`set(name, attributes["http.route"])`,
							`replace_match(attributes["http.target"], "/user/*/list/*", "/user/{userId}/list/{listId}")`,
							`limit(attributes, 100, [])`,
							`truncate_all(attributes, 4096)`,
						},
					},
				},
				"metric_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`keep_keys(attributes, ["host.name"])`,
							`truncate_all(attributes, 4096)`,
						},
					},
					map[string]any{
						"context": "metric",
						"statements": []any{
							`set(description, "Sum") where type == "Sum"`,
							`convert_sum_to_gauge() where name == "system.processes.count"`,
							`convert_gauge_to_sum("cumulative", false) where name == "prometheus_metric"`,
							`aggregate_on_attributes("sum") where name == "system.memory.usage"`,
						},
					},
					map[string]any{
						"context": "datapoint",
						"statements": []any{
							`limit(attributes, 100, ["host.name"])`,
							`truncate_all(attributes, 4096)`,
						},
					},
				},
				"log_statements": []any{
					map[string]any{
						"context": "resource",
						"statements": []any{
							`keep_keys(attributes, ["service.name", "service.namespace", "cloud.region"])`,
						},
					},
					map[string]any{
						"context": "log",
						"statements": []any{
							`set(severity_text, "FAIL") where body == "request failed"`,
							`replace_all_matches(attributes, "/user/*/list/*", "/user/{userId}/list/{listId}")`,
							`replace_all_patterns(attributes, "value", "/account/\\d{4}", "/account/{accountId}")`,
							`set(body, attributes["http.route"])`,
						},
					},
				},
				"profile_statements": []any{},
			},
		},
		{
			testName: "ManyStatements2",
			cfg: `
			trace_statements {
				context = "span"
				statements = [
					` + backtick + `set(name, "bear") where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			trace_statements {
				context = "resource"
				statements = [
					` + backtick + `set(attributes["name"], "bear")` + backtick + `,
				]
			}
			metric_statements {
				context = "datapoint"
				statements = [
					` + backtick + `set(metric.name, "bear") where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			metric_statements {
				context = "resource"
				statements = [
					` + backtick + `set(attributes["name"], "bear")` + backtick + `,
				]
			}
			log_statements {
				context = "log"
				statements = [
					` + backtick + `set(body, "bear") where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			log_statements {
				context = "resource"
				statements = [
					` + backtick + `set(attributes["name"], "bear")` + backtick + `,
				]
			}
			output {}
			`,
			expected: map[string]any{
				"error_mode": "propagate",
				"trace_statements": []any{
					map[string]any{
						"context": "span",
						"statements": []any{
							`set(name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				"metric_statements": []any{
					map[string]any{
						"context": "datapoint",
						"statements": []any{
							`set(metric.name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				"log_statements": []any{
					map[string]any{
						"context": "log",
						"statements": []any{
							`set(body, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					map[string]any{
						"context": "resource",
						"statements": []any{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				"profile_statements": []any{},
			},
		},
		{
			testName: "unknown_error_mode",
			cfg: `
			error_mode = "test"
			output {}
			`,
			errorMsg: `2:17: "test" unknown error mode test`,
		},
		{
			testName: "bad_syntax_log",
			cfg: `
			log_statements {
				context = "log"
				statements = [
					` + backtick + `set(body, "bear" where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `statement has invalid syntax: 1:18: unexpected token "where" (expected ")" Key*)`,
		},
		{
			testName: "bad_syntax_metric",
			cfg: `
			metric_statements {
				context = "datapoint"
				statements = [
					` + backtick + `set(name, "bear" where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `statement has invalid syntax: 1:18: unexpected token "where" (expected ")" Key*)`,
		},
		{
			testName: "bad_syntax_trace",
			cfg: `
			trace_statements {
				context = "span"
				statements = [
					` + backtick + `set(name, "bear" where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `keep_keys(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `statement has invalid syntax: 1:18: unexpected token "where" (expected ")" Key*)`,
		},
		{
			testName: "unknown_function_log",
			cfg: `
			log_statements {
				context = "log"
				statements = [
					` + backtick + `set(body, "bear") where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `not_a_function(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `unable to parse OTTL statement "not_a_function(log.attributes, [\"http.method\", \"http.path\"])": undefined function "not_a_function"`,
		},
		{
			testName: "unknown_function_metric",
			cfg: `
			metric_statements {
				context = "datapoint"
				statements = [
					` + backtick + `set(metric.name, "bear") where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `not_a_function(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `unable to parse OTTL statement "not_a_function(datapoint.attributes, [\"http.method\", \"http.path\"])": undefined function "not_a_function"`,
		},
		{
			testName: "unknown_function_trace",
			cfg: `
			trace_statements {
				context = "span"
				statements = [
					` + backtick + `set(name, "bear") where attributes["http.path"] == "/animal"` + backtick + `,
					` + backtick + `not_a_function(attributes, ["http.method", "http.path"])` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `unable to parse OTTL statement "not_a_function(span.attributes, [\"http.method\", \"http.path\"])": undefined function "not_a_function"`,
		},
		{
			testName: "unknown_context",
			cfg: `
			trace_statements {
				context = "test"
				statements = [
					` + backtick + `set(name, "bear") where attributes["http.path"] == "/animal"` + backtick + `,
				]
			}
			output {}
			`,
			errorMsg: `3:15: "test" unknown context test`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args transform.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}

			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*transformprocessor.Config)

			var expectedCfg transformprocessor.Config
			err = mapstructure.Decode(tc.expected, &expectedCfg)
			require.NoError(t, err)

			// Validate the two configs
			require.NoError(t, actual.Validate())
			// Don't validate expectedCfg, because it contains internal slices
			// with OTTL functions that aren't part of the config -
			// they are just a way to store internal state.
			// The validation would fail because those functions won't be registered.
			// You'd have to register those functions by creating a factory first.

			// Compare the two configs by marshaling to JSON.
			testutils.CompareConfigsAsJSON(t, actual, &expectedCfg)
		})
	}
}
