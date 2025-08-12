package mysql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/grafana/alloy/syntax"
)

func Test_collectSQLText(t *testing.T) {
	t.Run("enable sql text when provided", func(t *testing.T) {
		t.Parallel()

		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		disable_query_redaction = true
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.True(t, args.DisableQueryRedaction)
	})

	t.Run("disable sql text when not provided (default behavior)", func(t *testing.T) {
		t.Parallel()

		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.False(t, args.DisableQueryRedaction)
	})

	t.Run("setup consumers scrape interval is correctly parsed from config", func(t *testing.T) {
		t.Parallel()

		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		setup_consumers_collect_interval = "1h"
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Equal(t, time.Hour, args.SetupConsumersCollectInterval)
	})
}

func Test_parseCloudProvider(t *testing.T) {
	t.Run("parse cloud provider block", func(t *testing.T) {
		t.Parallel()

		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		disable_query_redaction = true
		cloud_provider {
			aws {
				arn = "arn:aws:rds:some-region:some-account:db:some-db-instance"
			}
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Equal(t, "arn:aws:rds:some-region:some-account:db:some-db-instance", args.CloudProvider.AWS.ARN)
	})
	t.Run("empty cloud provider block", func(t *testing.T) {
		t.Parallel()

		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		disable_query_redaction = true
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Nil(t, args.CloudProvider)
	})
}

func Test_enableOrDisableCollectors(t *testing.T) {
	t.Run("nothing specified (default behavior)", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName:    true,
			collector.SchemaTableName:    true,
			collector.QuerySampleName:    true,
			collector.SetupConsumersName: true,
			collector.ExplainPlanName:    false,
			collector.LocksName:          false,
		}, actualCollectors)
	})

	t.Run("enable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		enable_collectors = ["query_tables", "schema_table", "query_sample", "setup_consumers", "explain_plan", "locks"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName:    true,
			collector.SchemaTableName:    true,
			collector.QuerySampleName:    true,
			collector.SetupConsumersName: true,
			collector.ExplainPlanName:    true,
			collector.LocksName:          true,
		}, actualCollectors)
	})

	t.Run("disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["query_tables", "schema_table", "query_sample", "setup_consumers", "explain_plan"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName:    false,
			collector.SchemaTableName:    false,
			collector.QuerySampleName:    false,
			collector.SetupConsumersName: false,
			collector.ExplainPlanName:    false,
			collector.LocksName:          false,
		}, actualCollectors)
	})

	t.Run("enable collectors takes precedence over disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["query_tables", "schema_table", "query_sample", "setup_consumers", "explain_plan", "locks"]
		enable_collectors = ["query_tables", "schema_table", "query_sample", "setup_consumers", "explain_plan", "locks"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName:    true,
			collector.SchemaTableName:    true,
			collector.QuerySampleName:    true,
			collector.SetupConsumersName: true,
			collector.ExplainPlanName:    true,
			collector.LocksName:          true,
		}, actualCollectors)
	})

	t.Run("enabling one and disabling others", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["schema_table", "query_sample", "setup_consumers", "explain_plan", "locks"]
		enable_collectors = ["query_tables"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName:    true,
			collector.SchemaTableName:    false,
			collector.QuerySampleName:    false,
			collector.SetupConsumersName: false,
			collector.ExplainPlanName:    false,
			collector.LocksName:          false,
		}, actualCollectors)
	})

	t.Run("unknown collectors are ignored", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		enable_collectors = ["some_string"]
		disable_collectors = ["another_string"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName:    true,
			collector.SchemaTableName:    true,
			collector.QuerySampleName:    true,
			collector.SetupConsumersName: true,
			collector.ExplainPlanName:    false,
			collector.LocksName:          false,
		}, actualCollectors)
	})
}
