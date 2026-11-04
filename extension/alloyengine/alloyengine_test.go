package alloyengine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component"
)

// TestIncludesNativeComponents makes sure that OCB distributions which embed
// the extension get every native Alloy component.
func TestIncludesNativeComponents(t *testing.T) {
	bb, err := os.ReadFile("../../internal/component/all/testdata/components.yaml")
	require.NoError(t, err)
	var expected []string
	require.NoError(t, yaml.Unmarshal(bb, &expected))

	require.Equal(t, expected, component.AllNames())
}
