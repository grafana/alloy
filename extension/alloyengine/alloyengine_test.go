package alloyengine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
)

// TestIncludesNativeComponents makes sure that OCB distributions which embed
// the extension get every native Alloy component.
func TestIncludesNativeComponents(t *testing.T) {
	for _, name := range []string{"prometheus.scrape", "loki.write", "otelcol.receiver.otlp"} {
		_, ok := component.Get(name)
		require.True(t, ok, "component %s is not registered", name)
	}
	require.Equal(t, "alloyengine", NewFactory().Type().String())
}
