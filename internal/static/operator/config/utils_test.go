package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_unmarshalYAML(t *testing.T) {
	in := `
- a: 5
`

	out, err := unmarshalYAML([]interface{}{in})
	require.NoError(t, err)

	bb, err := yaml.Marshal(out)
	require.NoError(t, err)

	require.YAMLEq(t, in, string(bb))
}
