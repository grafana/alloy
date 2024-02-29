package snowflake_exporter

import (
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig_UnmarshalYaml(t *testing.T) {
	strConfig := `
account_name: "some_account"
username: "some_user"
password: "some_password"
warehouse: "some_warehouse"`

	var c Config

	require.NoError(t, yaml.UnmarshalStrict([]byte(strConfig), &c))

	require.Equal(t, Config{
		AccountName: "some_account",
		Username:    "some_user",
		Password:    "some_password",
		Warehouse:   "some_warehouse",
		Role:        "ACCOUNTADMIN",
	}, c)
}

func TestConfig_NewIntegration(t *testing.T) {
	t.Run("integration with valid config", func(t *testing.T) {
		c := &Config{
			AccountName: "some_account",
			Username:    "some_user",
			Password:    "some_password",
			Warehouse:   "some_warehouse",
			Role:        "ACCOUNTADMIN",
		}

		i, err := c.NewIntegration(log.NewJSONLogger(os.Stdout))
		require.NoError(t, err)
		require.NotNil(t, i)
	})

	t.Run("integration with invalid config", func(t *testing.T) {
		c := &Config{
			Username:  "some_user",
			Password:  "some_password",
			Warehouse: "some_warehouse",
			Role:      "ACCOUNTADMIN",
		}

		i, err := c.NewIntegration(log.NewJSONLogger(os.Stdout))
		require.Nil(t, i)
		require.ErrorContains(t, err, "")
	})
}

func TestConfig_AgentKey(t *testing.T) {
	c := Config{
		AccountName: "snowflake-acct",
	}

	ik, err := c.InstanceKey("agent-key")

	require.NoError(t, err)
	require.Equal(t, "snowflake-acct", ik)
}
