package convertparity_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/processor/tail_sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
)

func TestTailSampling(t *testing.T) {
	run[tail_sampling.Arguments](t, tailsamplingprocessor.NewFactory(), []tc{
		{name: "minimal", alloy: `
			policy {
				name = "all"
				type = "always_sample"
			}
			output {}`,
			collect: map[string]any{"policies": []any{map[string]any{"name": "all", "type": "always_sample"}}}},
		{name: "top-level settings", alloy: `
			decision_wait                     = "10s"
			decision_wait_after_root_received = "2s"
			num_traces                        = 100
			block_on_overflow                 = true
			expected_new_traces_per_sec       = 10
			sample_on_first_match             = true
			drop_pending_traces_on_shutdown   = true
			maximum_trace_size_bytes          = 1024
			num_shards                        = 4
			decision_cache = {
				sampled_cache_size     = 1000,
				non_sampled_cache_size = 2000,
			}
			policy {
				name = "all"
				type = "always_sample"
			}
			output {}`,
			collect: map[string]any{
				"decision_wait":                     "10s",
				"decision_wait_after_root_received": "2s",
				"num_traces":                        100,
				"block_on_overflow":                 true,
				"expected_new_traces_per_sec":       10,
				"sample_on_first_match":             true,
				"drop_pending_traces_on_shutdown":   true,
				"maximum_trace_size_bytes":          1024,
				"num_shards":                        4,
				"decision_cache":                    map[string]any{"sampled_cache_size": 1000, "non_sampled_cache_size": 2000},
				"policies":                          []any{map[string]any{"name": "all", "type": "always_sample"}},
			}},
		{name: "leaf policies", alloy: `
			policy {
				name = "latency"
				type = "latency"
				latency {
					threshold_ms       = 100
					upper_threshold_ms = 200
				}
			}
			policy {
				name = "numeric"
				type = "numeric_attribute"
				numeric_attribute {
					key          = "k"
					min_value    = 1
					max_value    = 2
					invert_match = true
				}
			}
			policy {
				name = "prob"
				type = "probabilistic"
				probabilistic {
					hash_salt           = "s"
					sampling_percentage = 12.5
				}
			}
			policy {
				name = "status"
				type = "status_code"
				status_code { status_codes = ["ERROR"] }
			}
			policy {
				name = "string"
				type = "string_attribute"
				string_attribute {
					key                    = "k"
					values                 = ["a.*"]
					enabled_regex_matching = true
					cache_max_size         = 5
					invert_match           = true
				}
			}
			policy {
				name = "rate"
				type = "rate_limiting"
				rate_limiting {
					spans_per_second = 10
					burst_capacity   = 20
				}
			}
			policy {
				name = "bytes"
				type = "bytes_limiting"
				bytes_limiting {
					bytes_per_second = 10
					burst_capacity   = 20
				}
			}
			policy {
				name = "spans"
				type = "span_count"
				span_count {
					min_spans = 1
					max_spans = 9
				}
			}
			policy {
				name = "bool"
				type = "boolean_attribute"
				boolean_attribute {
					key          = "k"
					value        = true
					invert_match = true
				}
			}
			policy {
				name = "ottl"
				type = "ottl_condition"
				ottl_condition {
					error_mode = "ignore"
					span       = ["true"]
					spanevent  = ["false"]
				}
			}
			policy {
				name = "ts"
				type = "trace_state"
				trace_state {
					key    = "k"
					values = ["v"]
				}
			}
			output {}`,
			collect: map[string]any{"policies": []any{
				map[string]any{"name": "latency", "type": "latency", "latency": map[string]any{"threshold_ms": 100, "upper_threshold_ms": 200}},
				map[string]any{"name": "numeric", "type": "numeric_attribute", "numeric_attribute": map[string]any{"key": "k", "min_value": 1, "max_value": 2, "invert_match": true}},
				map[string]any{"name": "prob", "type": "probabilistic", "probabilistic": map[string]any{"hash_salt": "s", "sampling_percentage": 12.5}},
				map[string]any{"name": "status", "type": "status_code", "status_code": map[string]any{"status_codes": []any{"ERROR"}}},
				map[string]any{"name": "string", "type": "string_attribute", "string_attribute": map[string]any{"key": "k", "values": []any{"a.*"}, "enabled_regex_matching": true, "cache_max_size": 5, "invert_match": true}},
				map[string]any{"name": "rate", "type": "rate_limiting", "rate_limiting": map[string]any{"spans_per_second": 10, "burst_capacity": 20}},
				map[string]any{"name": "bytes", "type": "bytes_limiting", "bytes_limiting": map[string]any{"bytes_per_second": 10, "burst_capacity": 20}},
				map[string]any{"name": "spans", "type": "span_count", "span_count": map[string]any{"min_spans": 1, "max_spans": 9}},
				map[string]any{"name": "bool", "type": "boolean_attribute", "boolean_attribute": map[string]any{"key": "k", "value": true, "invert_match": true}},
				map[string]any{"name": "ottl", "type": "ottl_condition", "ottl_condition": map[string]any{"error_mode": "ignore", "span": []any{"true"}, "spanevent": []any{"false"}}},
				map[string]any{"name": "ts", "type": "trace_state", "trace_state": map[string]any{"key": "k", "values": []any{"v"}}},
			}}},
		{name: "and, not and drop", alloy: `
			policy {
				name = "and"
				type = "and"
				and {
					and_sub_policy {
						name = "a1"
						type = "latency"
						latency { threshold_ms = 5 }
					}
					and_sub_policy {
						name = "a2"
						type = "ottl_condition"
						ottl_condition {
							error_mode = "propagate"
							span       = ["true"]
						}
					}
				}
			}
			policy {
				name = "not"
				type = "not"
				not {
					not_sub_policy {
						name = "n1"
						type = "status_code"
						status_code { status_codes = ["OK"] }
					}
				}
			}
			policy {
				name = "drop"
				type = "drop"
				drop {
					drop_sub_policy {
						name = "d1"
						type = "always_sample"
					}
				}
			}
			output {}`,
			collect: map[string]any{"policies": []any{
				map[string]any{"name": "and", "type": "and", "and": map[string]any{"and_sub_policy": []any{
					map[string]any{"name": "a1", "type": "latency", "latency": map[string]any{"threshold_ms": 5}},
					map[string]any{"name": "a2", "type": "ottl_condition", "ottl_condition": map[string]any{"error_mode": "propagate", "span": []any{"true"}}},
				}}},
				map[string]any{"name": "not", "type": "not", "not": map[string]any{"not_sub_policy": map[string]any{
					"name": "n1", "type": "status_code", "status_code": map[string]any{"status_codes": []any{"OK"}},
				}}},
				map[string]any{"name": "drop", "type": "drop", "drop": map[string]any{"drop_sub_policy": []any{
					map[string]any{"name": "d1", "type": "always_sample"},
				}}},
			}}},
		{name: "composite", alloy: `
			policy {
				name = "c"
				type = "composite"
				composite {
					max_total_spans_per_second = 100
					policy_order               = ["s1", "s2"]
					composite_sub_policy {
						name = "s1"
						type = "always_sample"
					}
					composite_sub_policy {
						name = "s2"
						type = "and"
						and {
							and_sub_policy {
								name = "s2a"
								type = "span_count"
								span_count { min_spans = 2 }
							}
						}
					}
					rate_allocation {
						policy  = "s1"
						percent = 60
					}
					rate_allocation {
						policy  = "s2"
						percent = 40
					}
				}
			}
			output {}`,
			collect: map[string]any{"policies": []any{
				map[string]any{"name": "c", "type": "composite", "composite": map[string]any{
					"max_total_spans_per_second": 100,
					"policy_order":               []any{"s1", "s2"},
					"composite_sub_policy": []any{
						map[string]any{"name": "s1", "type": "always_sample"},
						map[string]any{"name": "s2", "type": "and", "and": map[string]any{"and_sub_policy": []any{
							map[string]any{"name": "s2a", "type": "span_count", "span_count": map[string]any{"min_spans": 2}},
						}}},
					},
					"rate_allocation": []any{
						map[string]any{"policy": "s1", "percent": 60},
						map[string]any{"policy": "s2", "percent": 40},
					},
				}},
			}}},
	})
}
