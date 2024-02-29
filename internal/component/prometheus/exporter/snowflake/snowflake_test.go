package snowflake

import (
	"testing"

	"github.com/grafana/agent/internal/static/integrations/snowflake_exporter"
	"github.com/grafana/river"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	riverConfig := `
	account_name = "some_account"
	username     = "some_user"
	password     = "some_password"
	role         = "some_role"
	warehouse    = "some_warehouse"
	`

	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		AccountName: "some_account",
		Username:    "some_user",
		Password:    rivertypes.Secret("some_password"),
		Role:        "some_role",
		Warehouse:   "some_warehouse",
	}

	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	riverConfig := `
	account_name = "some_account"
	username     = "some_user"
	password     = "some_password"
	warehouse    = "some_warehouse"
	`
	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := snowflake_exporter.Config{
		AccountName: "some_account",
		Username:    "some_user",
		Password:    config_util.Secret("some_password"),
		Role:        DefaultArguments.Role,
		Warehouse:   "some_warehouse",
	}
	require.Equal(t, expected, *res)
}
