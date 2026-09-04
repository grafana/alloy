//go:build alloy_custom_components

package converter

import (
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildDisablesConverters(t *testing.T) {
	require.Empty(t, SupportedFormats)

	result, diags := Convert([]byte("scrape_configs: []"), InputPrometheus, nil)
	require.Nil(t, result)
	require.Len(t, diags, 1)
	require.Equal(t, diag.SeverityLevelCritical, diags[0].Severity)
	require.True(t, strings.Contains(diags[0].Summary, "not included in this custom component build"))
}
