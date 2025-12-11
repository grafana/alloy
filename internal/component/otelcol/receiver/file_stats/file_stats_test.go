//go:build linux || darwin || windows

package file_stats_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/file_stats"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArguments(t *testing.T) {
	in := `
		include = "/var/log/*"

	  metrics {
			file.atime {
				enabled = true
			}
			file.count {
				enabled = true
			}
		}

		resource_attributes {
			file.name {
				enabled = true

				metrics_include {
					strict = "foobar"
				}
				metrics_include {
					strict = "foobar2"
				}

				metrics_exclude {
					regexp = "fizz.*"
				}
			}
		}

		output {
			// no-op
		}
	`

	var args file_stats.Arguments
	err := syntax.Unmarshal([]byte(in), &args)
	require.NoError(t, err, "arguments should unmarshal without error")

	outAny, err := args.Convert()
	require.NoError(t, err, "Arguments should not fail to convert")

	out := outAny.(*filestatsreceiver.Config)

	// We can't compare the types at a high level because the upstream type has
	// fields in an internal package, so we check some fields individually here.
	//
	// NOTE(rfratto): we don't check for defaults being applied; we're primarily
	// only interested in ensuring the conversion works.
	assert.Equal(t, "/var/log/*", out.Include)
	assert.Equal(t, true, out.Metrics.FileAtime.Enabled)
	assert.Equal(t, true, out.Metrics.FileCount.Enabled)
	assert.Equal(t, true, out.ResourceAttributes.FileName.Enabled)
	assert.Equal(t, "foobar", out.ResourceAttributes.FileName.MetricsInclude[0].Strict)
	assert.Equal(t, "foobar2", out.ResourceAttributes.FileName.MetricsInclude[1].Strict)
	assert.Equal(t, "fizz.*", out.ResourceAttributes.FileName.MetricsExclude[0].Regex)
}

func TestArguments_NoFilters(t *testing.T) {
	in := `
		include = "/var/log/*"

	  metrics {
			file.atime {
				enabled = true
			}
			file.count {
				enabled = true
			}
		}

		resource_attributes {
			file.name {
				enabled = true
			}
		}

		output {
			// no-op
		}
	`

	var args file_stats.Arguments
	err := syntax.Unmarshal([]byte(in), &args)
	require.NoError(t, err, "arguments should unmarshal without error")

	outAny, err := args.Convert()
	require.NoError(t, err, "Arguments should not fail to convert")

	out := outAny.(*filestatsreceiver.Config)

	// NOTE(rfratto): filestatsreceiver 0.99 creates a filter if MetricsInclude
	// and MetricsExclude are non-nil, even if they are completely empty; this
	// means we _must_ set them to nil if they are empty otherwise everything
	// will be filtered out.
	if assert.Len(t, out.ResourceAttributes.FileName.MetricsInclude, 0, "Expected MetricsInclude to be len 0") {
		assert.Nil(t, out.ResourceAttributes.FileName.MetricsInclude, "MetricsInclude must be nil when empty")
	}
	if assert.Len(t, out.ResourceAttributes.FileName.MetricsExclude, 0, "Expected MetricsExclude to be len 0") {
		assert.Nil(t, out.ResourceAttributes.FileName.MetricsExclude, "MetricsExclude must be nil when empty")
	}
}
