package snmp_exporter_v2

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestSnmpConfig(t *testing.T) {
	t.Run("reload unmarshals", func(t *testing.T) {
		var config Config

		strConfig := `---
walk_params:
  keyone:
`

		// The first time should not return any errors.
		require.NoError(t, yaml.UnmarshalStrict([]byte(strConfig), &config), "initial unmarshal")
		require.Len(t, config.WalkParams, 1)

		// A second time (executed on reload), the map will already have the specified key(s), but should still succeed
		require.NoError(t, yaml.UnmarshalStrict([]byte(strConfig), &config), "reload unmarshal")
		require.Len(t, config.WalkParams, 1)
	})
}
