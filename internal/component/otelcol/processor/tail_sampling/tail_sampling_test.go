package tail_sampling

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestBadAlloyConfig(t *testing.T) {
	exampleBadAlloyConfig := `
    decision_wait               = "10s"
    num_traces                  = 0
    expected_new_traces_per_sec = 10
    policy {
      name = "test-policy-1"
      type = "always_sample"
    }
    output { 
	    // no-op: will be overridden by test code.
    }
`

	var args Arguments
	require.Error(t, syntax.Unmarshal([]byte(exampleBadAlloyConfig), &args), "num_traces must be greater than zero")
}

func TestBadAlloyConfigErrorMode(t *testing.T) {
	exampleBadAlloyConfig := `
    decision_wait               = "10s"
    num_traces                  = 5
    expected_new_traces_per_sec = 10
    policy {
      name = "test-policy-1"
      type = "ottl_condition"
      ottl_condition {
        error_mode = ""
        span = [
          "attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"",
          "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\"",
        ]
        spanevent = [
          "name != \"test_span_event_name\"",
          "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\"",
        ]
      }
    }
    output { 
	    // no-op: will be overridden by test code.
    }
`

	var args Arguments
	require.ErrorContains(t, syntax.Unmarshal([]byte(exampleBadAlloyConfig), &args), "\"\" unknown error mode")
}

func TestBadOtelConfig(t *testing.T) {
	var exampleBadOtelConfig = `
    decision_wait               = "10s"
    num_traces                  = 100
    expected_new_traces_per_sec = 10
    policy {
      name = "test-policy-1"
      type = "bad_type"
    }
    output { 
      // no-op: will be overridden by test code.
    }
`

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.tail_sampling")
	require.NoError(t, err)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(exampleBadOtelConfig), &args))

	// Override our arguments so traces get forwarded to traceCh.
	traceCh := make(chan ptrace.Traces)
	args.Output = makeTracesOutput(traceCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.Error(t, err, "unknown sampling policy type bad_type")
	}()

	require.Error(t, ctrl.WaitRunning(time.Second), "component never started")
}

func TestBigConfig(t *testing.T) {
	exampleBigConfig := `
    decision_wait               = "10s"
    num_traces                  = 100
    expected_new_traces_per_sec = 10
    policy {
      name = "test-policy-1"
      type = "always_sample"
    }
    policy {
      name = "test-policy-2"
    	type = "latency"
      latency {
        threshold_ms = 5000
      }
    }
    policy {
      name = "test-policy-3"
      type = "numeric_attribute"
      numeric_attribute {
        key = "key1"
        min_value = 50
        max_value = 100
      }
    }
    policy {
      name = "test-policy-4"
      type = "probabilistic"
      probabilistic {
        sampling_percentage = 10
      }
    }
    policy {
      name = "test-policy-5"
      type = "status_code"
      status_code {
        status_codes = ["ERROR", "UNSET"]
      }
    }
    policy {
      name = "test-policy-6"
      type = "string_attribute"
      string_attribute {
        key = "key2"
        values = ["value1", "value2"]
      }
    }
    policy {
      name = "test-policy-7"
      type = "string_attribute"
      string_attribute {
        key = "key2"
        values = ["value1", "val*"]
        enabled_regex_matching = true
        cache_max_size = 10
      }
    }
    policy {
      name = "test-policy-8"
      type = "rate_limiting"
      rate_limiting {
        spans_per_second = 35
      }
    }
    policy {
      name = "test-policy-9"
      type = "string_attribute"
      string_attribute {
        key = "http.url"
        values = ["/health", "/metrics"]
        enabled_regex_matching = true
        invert_match = true
      }
    }
    policy {
      name = "test-policy-10"
      type = "span_count"
      span_count {
        min_spans = 2
      }
    }
    policy {
      name = "test-policy-11"
      type = "trace_state"
      trace_state {
        key = "key3"
        values = ["value1", "value2"]
      }
    }
    policy {
      name = "test-policy-12"
      type = "boolean_attribute"
      boolean_attribute {
        key = "key4"
        value = true
        invert_match = true
      }
    }
    policy {
      name = "test-policy-13"
      type = "ottl_condition"
      ottl_condition {
        error_mode = "ignore"
        span = [
          "attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"",
          "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\"",
        ]
        spanevent = [
          "name != \"test_span_event_name\"",
          "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\"",
        ]
      }
    }
    policy {
      name = "test-policy-14"
      type = "ottl_condition"
      ottl_condition {
        error_mode = "ignore"
        span = [
          "attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"",
          "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\"",
        ]
      }
    }
    policy {
      name = "test-policy-15"
      type = "ottl_condition"
      ottl_condition {
        error_mode = "propagate"
        spanevent = [
          "name != \"test_span_event_name\"",
          "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\"",
        ]
      }
    }
    policy{
      name = "and-policy-1"
      type = "and"
      and {
        and_sub_policy {
          name = "test-and-policy-1"
          type = "numeric_attribute"
          numeric_attribute {
            key = "key1"
            min_value = 50
            max_value = 100
          }
        }
        and_sub_policy {
          name = "test-and-policy-2"
          type = "string_attribute"
          string_attribute {
            key = "key1"
            values = ["value1", "value2"]
          }
        }
        and_sub_policy {
          name = "test-and-policy-3"
          type = "boolean_attribute"
          boolean_attribute {
            key = "key4"
            value = true
          }
        }
        and_sub_policy {
          name = "test-and-policy-4"
          type = "ottl_condition"
          ottl_condition {
            error_mode = "ignore"
            span = [
              "attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"",
              "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\"",
            ]
            spanevent = [
              "name != \"test_span_event_name\"",
              "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\"",
            ]
          }
        }
      }
    }
    policy{
      name = "composite-policy-1"
      type = "composite"
      composite {
        max_total_spans_per_second = 1000
        policy_order = ["test-composite-policy-1", "test-composite-policy-2", "test-composite-policy-3"]
        composite_sub_policy {
          name = "test-composite-policy-1"
          type = "numeric_attribute"
          numeric_attribute {
            key = "key1"
            min_value = 50
            max_value = 100
          }
        }
        composite_sub_policy {
          name = "test-composite-policy-2"
          type = "string_attribute"
          string_attribute {
            key = "key1"
            values = ["value1", "value2"]
          }
        }
        composite_sub_policy {
          name = "test-composite-policy-3"
          type = "always_sample"
        }
        composite_sub_policy {
          name = "test-composite-policy-4"
          type = "boolean_attribute"
          boolean_attribute {
            key = "key4"
            value = true
          }
        }
        composite_sub_policy {
          name = "test-composite-policy-5"
          type = "ottl_condition"
          ottl_condition {
            error_mode = "ignore"
            span = [
              "attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"",
              "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\"",
            ]
            spanevent = [
              "name != \"test_span_event_name\"",
              "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\"",
            ]
          }
        }
        rate_allocation {
          policy = "test-composite-policy-1"
          percent = 50
        }
        rate_allocation {
          policy = "test-composite-policy-2"
          percent = 50
        }
      }
    }
  
    output {
  	  // no-op: will be overridden by test code.
    }
`

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.tail_sampling")
	require.NoError(t, err)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(exampleBigConfig), &args))

	// Override our arguments so traces get forwarded to traceCh.
	traceCh := make(chan ptrace.Traces)
	args.Output = makeTracesOutput(traceCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")
}

func TestTraceProcessing(t *testing.T) {
	exampleSmallConfig := `
    decision_wait               = "1s"
    num_traces                  = 1
    expected_new_traces_per_sec = 1
    policy {
      name = "test-policy-1"
      type = "always_sample"
    }
    output { 
	    // no-op: will be overridden by test code.
    }
  `
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.tail_sampling")
	require.NoError(t, err)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(exampleSmallConfig), &args))

	// Override our arguments so traces get forwarded to traceCh.
	traceCh := make(chan ptrace.Traces)
	args.Output = makeTracesOutput(traceCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	// Send traces in the background to our processor.
	go func() {
		exports := ctrl.Exports().(otelcol.ConsumerExports)

		exports.Input.Capabilities()

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			err := exports.Input.ConsumeTraces(ctx, createTestTraces())
			if err != nil {
				level.Error(l).Log("msg", "failed to send traces", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our processor to finish and forward data to traceCh.
	select {
	case <-time.After(time.Second * 10):
		require.FailNow(t, "failed waiting for traces")
	case tr := <-traceCh:
		require.Equal(t, 1, tr.SpanCount())
	}
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

func createTestTraces() ptrace.Traces {
	// Matches format from the protobuf definition:
	// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto
	var bb = `{
		"resource_spans": [{
			"scope_spans": [{
				"spans": [{
					"name": "TestSpan"
				}]
			}]
		}]
	}`

	decoder := &ptrace.JSONUnmarshaler{}
	data, err := decoder.UnmarshalTraces([]byte(bb))
	if err != nil {
		panic(err)
	}
	return data
}
