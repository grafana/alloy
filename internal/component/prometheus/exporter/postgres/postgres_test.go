package postgres

import (
	"testing"

	"github.com/grafana/alloy/internal/static/integrations/postgres_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfigUnmarshal(t *testing.T) {
	exampleAlloyConfig := `
	data_source_names = ["postgresql://username:password@localhost:5432/database?sslmode=disable"]
	disable_settings_metrics = true
	disable_default_metrics = true
	custom_queries_config_path = "/tmp/queries.yaml"

	stat_statements {
		include_query = true
		query_length = 200
	}

	autodiscovery {
		enabled = false
		database_allowlist = ["include1"]
		database_denylist  = ["exclude1", "exclude2"]
	}`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		DataSourceNames:        []alloytypes.Secret{alloytypes.Secret("postgresql://username:password@localhost:5432/database?sslmode=disable")},
		DisableSettingsMetrics: true,
		AutoDiscovery: AutoDiscovery{
			Enabled:           false,
			DatabaseDenylist:  []string{"exclude1", "exclude2"},
			DatabaseAllowlist: []string{"include1"},
		},
		DisableDefaultMetrics:   true,
		CustomQueriesConfigPath: "/tmp/queries.yaml",
		StatStatementFlags: &StatStatementFlags{
			IncludeQuery: true,
			QueryLength:  200,
		},
	}

	require.Equal(t, expected, args)
}

func TestAlloyConfigConvert(t *testing.T) {
	exampleAlloyConfig := `
	data_source_names = ["postgresql://username:password@localhost:5432/database?sslmode=disable"]
	disable_settings_metrics = true
	disable_default_metrics = false
	custom_queries_config_path = "/tmp/queries.yaml"
	enabled_collectors = ["collector1", "collector2"]

	autodiscovery {
		enabled = false
		database_allowlist = ["include1"]
		database_denylist  = ["exclude1", "exclude2"]
	}`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	c := args.convert("test-instance")

	expected := postgres_exporter.Config{
		DataSourceNames:        []config_util.Secret{config_util.Secret("postgresql://username:password@localhost:5432/database?sslmode=disable")},
		DisableSettingsMetrics: true,
		AutodiscoverDatabases:  false,
		ExcludeDatabases:       []string{"exclude1", "exclude2"},
		IncludeDatabases:       []string{"include1"},
		DisableDefaultMetrics:  false,
		QueryPath:              "/tmp/queries.yaml",
		Instance:               "test-instance",
		EnabledCollectors:      []string{"collector1", "collector2"},
		StatStatementFlags:     nil,
	}
	require.Equal(t, expected, *c)
}

func TestRiverConfigValidate(t *testing.T) {
	tc := []struct {
		name        string
		args        Arguments
		expectedErr string
	}{
		{
			name: "no errors on default config",
			args: Arguments{},
		},
		{
			name: "missing custom queries file path",
			args: Arguments{
				DisableDefaultMetrics: true,
			},
			expectedErr: "custom_queries_config_path must be set when disable_default_metrics is true",
		},
		{
			name: "disabled default metrics with enabled collectors",
			args: Arguments{
				DisableDefaultMetrics:   true,
				CustomQueriesConfigPath: "/tmp/queries.yaml",
				EnabledCollectors:       []string{"collector1"},
			},
			expectedErr: "enabled_collectors cannot be set when disable_default_metrics is true",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}
