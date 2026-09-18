package groupbyattrs_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/processor/groupbyattrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args groupbyattrs.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &groupbyattrsprocessor.Config{GroupByKeys: []string{}}, cfg)
}
