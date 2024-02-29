package util

import (
	"testing"

	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

type NoSecretConfig struct {
	ValOne int    `yaml:"val_one,omitempty"`
	ValTwo string `yaml:"val_two,omitempty"`
}

type SecretConfig struct {
	Password config_util.Secret `yaml:"password,omitempty"`
	ValThree int                `yaml:"val_three,omitempty"`
}

func TestCompareEqualNoSecrets(t *testing.T) {
	s1 := NoSecretConfig{
		ValOne: 123,
		ValTwo: "456",
	}
	s2 := NoSecretConfig{
		ValOne: 123,
		ValTwo: "456",
	}
	require.True(t, CompareYAML(s1, s2))
}

func TestCompareNotEqualNoSecrets(t *testing.T) {
	s1 := NoSecretConfig{
		ValOne: 123,
		ValTwo: "321",
	}
	s2 := NoSecretConfig{
		ValOne: 123,
		ValTwo: "456",
	}
	require.False(t, CompareYAML(s1, s2))
}

func TestCompareEqualWithSecrets(t *testing.T) {
	s1 := SecretConfig{
		Password: config_util.Secret("pass"),
		ValThree: 3,
	}

	s2 := SecretConfig{
		Password: config_util.Secret("pass"),
		ValThree: 3,
	}
	require.True(t, CompareYAMLWithHook(s1, s2, noScrubbedSecretsHook))
}

func TestCompareNotEqualWithSecrets(t *testing.T) {
	s1 := SecretConfig{
		Password: config_util.Secret("pass"),
		ValThree: 3,
	}

	s2 := SecretConfig{
		Password: config_util.Secret("not_pass"),
		ValThree: 3,
	}
	require.False(t, CompareYAMLWithHook(s1, s2, noScrubbedSecretsHook))

	s3 := SecretConfig{
		Password: config_util.Secret("pass"),
		ValThree: 4,
	}
	require.False(t, CompareYAMLWithHook(s1, s3, noScrubbedSecretsHook))
}

func noScrubbedSecretsHook(in interface{}) (ok bool, out interface{}, err error) {
	switch v := in.(type) {
	case config_util.Secret:
		return true, string(v), nil
	case *config_util.URL:
		return true, v.String(), nil
	default:
		return false, nil, nil
	}
}
