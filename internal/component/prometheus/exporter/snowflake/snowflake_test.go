package snowflake

import (
	"testing"

	"github.com/grafana/alloy/internal/static/integrations/snowflake_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
	account_name 				 = "some_account"
	username     				 = "some_user"
	password     				 = "some_password"
	private_key_path 		 = "/some/path/rsa_key.p8"
	private_key_password = "some_password"
	role         				 = "some_role"
	warehouse    				 = "some_warehouse"
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		AccountName:        "some_account",
		Username:           "some_user",
		Password:           alloytypes.Secret("some_password"),
		PrivateKeyPath:     "/some/path/rsa_key.p8",
		PrivateKeyPassword: alloytypes.Secret("some_password"),
		Role:               "some_role",
		Warehouse:          "some_warehouse",
	}

	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	alloyConfig := `
	account_name         = "some_account"
	username             = "some_user"
	password             = "some_password"
	private_key_path     = "/some/path/rsa_key.p8"
	private_key_password = "some_password"
	warehouse            = "some_warehouse"
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := snowflake_exporter.Config{
		AccountName:        "some_account",
		Username:           "some_user",
		Password:           config_util.Secret("some_password"),
		PrivateKeyPath:     "/some/path/rsa_key.p8",
		PrivateKeyPassword: config_util.Secret("some_password"),
		Role:               DefaultArguments.Role,
		Warehouse:          "some_warehouse",
	}
	require.Equal(t, expected, *res)
}
