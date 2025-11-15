package flowcmd

import (
	"testing"

	"github.com/grafana/alloy/internal/build"
	"github.com/stretchr/testify/require"
)

func Test_fallbackVersionFromJSON(t *testing.T) {
	in := `{".": "1.2.3"}`
	expect := "v1.2.3-devel"

	actual := fallbackVersionFromJSON([]byte(in))
	require.Equal(t, expect, actual)
}

func Test_fallbackVersionFromJSON_InvalidJSON(t *testing.T) {
	in := `not valid json`
	// Should return build.Version when JSON is invalid
	actual := fallbackVersionFromJSON([]byte(in))
	require.Equal(t, build.Version, actual)
}

func Test_fallbackVersionFromJSON_MissingKey(t *testing.T) {
	in := `{"other": "1.2.3"}`
	// Should return build.Version when key is missing
	actual := fallbackVersionFromJSON([]byte(in))
	require.Equal(t, build.Version, actual)
}
