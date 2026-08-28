package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig_MarshalYAMLOmitEmptyFields(t *testing.T) {
	var cfg Config
	yml, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.Equal(t, "{}\n", string(yml))
}

func TestConfig_MarshalYAMLOmitDefaultConfigFields(t *testing.T) {
	cfg := DefaultConfig
	yml, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.NotContains(t, string(yml), "kvstore")
	require.NotContains(t, string(yml), "lifecycler")
}
