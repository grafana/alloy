package squid_exporter

import (
	"errors"
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name        string
		getConfig   func() Config
		expectedErr error
	}{
		{
			name: "valid",
			getConfig: func() Config {
				c := Config{
					Address: "localhost:3128",
				}
				return c
			},
		},
		{
			name: "no hostname",
			getConfig: func() Config {
				c := Config{}
				c.Address = ":3128"
				return c
			},
			expectedErr: ErrNoHostname,
		},
		{
			name: "no port",
			getConfig: func() Config {
				c := Config{}
				c.Address = "localhost:"
				return c
			},
			expectedErr: ErrNoPort,
		},
		{
			name: "no empty config",
			getConfig: func() Config {
				cfg := Config{
					Address:  "",
					Username: "",
					Password: "",
				}
				return cfg
			},
			expectedErr: ErrNoAddress,
		},
		{
			name: "invalid config",
			getConfig: func() Config {
				cfg := Config{}
				cfg.Address = "a@#$%:asdf::12312"
				return cfg
			},
			expectedErr: errors.New("address a@#$%:asdf::12312: too many colons in address"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.getConfig()
			err := cfg.validate()
			if tc.expectedErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.expectedErr.Error())
		})
	}
}

func TestConfig_UnmarshalYaml(t *testing.T) {
	t.Run("only required values", func(t *testing.T) {
		strConfig := `address: "localhost:3182"`

		var c Config

		require.NoError(t, yaml.UnmarshalStrict([]byte(strConfig), &c))

		require.Equal(t, Config{
			Address: "localhost:3182",
		}, c)
	})

	t.Run("all values", func(t *testing.T) {
		strConfig := `
address: "localhost:3182"
username: "user"
password: "password"`

		var c Config

		require.NoError(t, yaml.UnmarshalStrict([]byte(strConfig), &c))

		require.Equal(t, Config{
			Address:  "localhost:3182",
			Username: "user",
			Password: "password",
		}, c)
	})
}

func TestConfig_InstanceKey(t *testing.T) {
	c := Config{}
	c.Address = "localhost:3128"

	ik := "agent-key"
	id, err := c.InstanceKey(ik)
	require.NoError(t, err)
	require.Equal(t, "localhost:3128", id)
}

func TestConfig_NewIntegration(t *testing.T) {
	t.Run("integration with valid config", func(t *testing.T) {
		c := Config{
			Address: "localhost:3128",
		}

		i, err := c.NewIntegration(log.NewJSONLogger(os.Stdout))
		require.NoError(t, err)
		require.NotNil(t, i)
	})

	t.Run("integration with invalid config", func(t *testing.T) {
		c := Config{}

		i, err := c.NewIntegration(log.NewJSONLogger(os.Stdout))
		require.Nil(t, i)
		require.ErrorContains(t, err, "failed to validate config:")
	})
}
