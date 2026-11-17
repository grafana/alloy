//go:build linux || darwin || windows

package file_stats_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/file_stats"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
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
	assert.Equal(t, true, out.MetricsBuilderConfig.Metrics.FileAtime.Enabled)
	assert.Equal(t, true, out.MetricsBuilderConfig.Metrics.FileCount.Enabled)
	assert.Equal(t, true, out.MetricsBuilderConfig.ResourceAttributes.FileName.Enabled)
	assert.Equal(t, "foobar", out.MetricsBuilderConfig.ResourceAttributes.FileName.MetricsInclude[0].Strict)
	assert.Equal(t, "foobar2", out.MetricsBuilderConfig.ResourceAttributes.FileName.MetricsInclude[1].Strict)
	assert.Equal(t, "fizz.*", out.MetricsBuilderConfig.ResourceAttributes.FileName.MetricsExclude[0].Regex)
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
	if assert.Len(t, out.MetricsBuilderConfig.ResourceAttributes.FileName.MetricsInclude, 0, "Expected MetricsInclude to be len 0") {
		assert.Nil(t, out.MetricsBuilderConfig.ResourceAttributes.FileName.MetricsInclude, "MetricsInclude must be nil when empty")
	}
	if assert.Len(t, out.MetricsBuilderConfig.ResourceAttributes.FileName.MetricsExclude, 0, "Expected MetricsExclude to be len 0") {
		assert.Nil(t, out.MetricsBuilderConfig.ResourceAttributes.FileName.MetricsExclude, "MetricsExclude must be nil when empty")
	}
}

func TestDefaultArguments(t *testing.T) {
	var args file_stats.Arguments
	args.SetToDefault()

	cfgAny, err := args.Convert()
	require.NoError(t, err)
	cfg := cfgAny.(*filestatsreceiver.Config)

	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	// MetricsBuilderConfig is not covered: its type lives in an upstream internal package,
	// so it cannot be written out here.
	require.Equal(t, "", cfg.Include)
	require.Equal(t, scraperhelper.ControllerConfig{
		CollectionInterval: time.Minute,
		InitialDelay:       time.Second,
	}, cfg.ControllerConfig)
}
