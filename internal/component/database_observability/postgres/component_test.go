package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/syntax"
)

func Test_enableOrDisableCollectors(t *testing.T) {
	t.Run("nothing specified (default behavior)", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.ActivityName:    false,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("enable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["query_tables"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.ActivityName:    false,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		disable_collectors = ["query_tables"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.ActivityName:    false,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("enable collectors takes precedence over disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		disable_collectors = ["query_tables"]
		enable_collectors = ["query_tables"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.ActivityName:    false,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("unknown collectors are ignored", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["some_string"]
		disable_collectors = ["another_string"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.ActivityName:    false,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("enable activity collector", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["activity"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.ActivityName:    true,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("enable schema table", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["schema_table"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.ActivityName:    false,
			collector.SchemaTableName: true,
		}, actualCollectors)
	})

	t.Run("enable multiple collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["query_tables", "activity"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.ActivityName:    true,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("disable activity collector", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		disable_collectors = ["activity"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.ActivityName:    false,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})
}
