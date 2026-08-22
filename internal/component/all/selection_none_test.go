//go:build alloy_custom_components && alloy_test_custom_none

package all

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildWithNoComponents(t *testing.T) {
	require.Empty(t, component.AllNames())
}
