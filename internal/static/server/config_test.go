package server

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig_OmitEmptyFields(t *testing.T) {
	var cfg Config
	yml, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.Equal(t, "{}\n", string(yml))
}
