package mysql

import (
	"testing"

	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func Test_collectSQLText(t *testing.T) {
	t.Run("enable sql text when provided", func(t *testing.T) {
		t.Parallel()

		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		collect_sql_text = true
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.True(t, args.CollectSQLText)
	})

	t.Run("disable sql text when not provided (default behavior)", func(t *testing.T) {
		t.Parallel()

		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.False(t, args.CollectSQLText)
	})
}

func Test_enableOrDisableCollectors(t *testing.T) {
	t.Run("nothing specified (default behavior)", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.SchemaTableName: true,
			collector.QuerySampleName: false,
		}, actualCollectors)
	})

	t.Run("enable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		enable_collectors = ["query_tables", "schema_table", "query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.SchemaTableName: true,
			collector.QuerySampleName: true,
		}, actualCollectors)
	})

	t.Run("disable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["query_tables", "schema_table", "query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.SchemaTableName: false,
			collector.QuerySampleName: false,
		}, actualCollectors)
	})

	t.Run("enable collectors takes precedence over disable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["query_tables", "schema_table", "query_sample"]
		enable_collectors = ["query_tables", "schema_table", "query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.SchemaTableName: true,
			collector.QuerySampleName: true,
		}, actualCollectors)
	})

	t.Run("enabling one and disabling others", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["schema_table", "query_sample"]
		enable_collectors = ["query_tables"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.SchemaTableName: false,
			collector.QuerySampleName: false,
		}, actualCollectors)
	})

	t.Run("unknown collectors are ignored", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
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
			collector.QueryTablesName: true,
			collector.SchemaTableName: true,
			collector.QuerySampleName: false,
		}, actualCollectors)
	})
}
