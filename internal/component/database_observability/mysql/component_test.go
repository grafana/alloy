package mysql

import (
	"testing"

	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

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
			collector.QuerySampleName: true,
			collector.SchemaTableName: true,
		}, actualCollectors)
	})

	t.Run("enable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		enable_collectors = ["query_sample", "schema_table"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QuerySampleName: true,
			collector.SchemaTableName: true,
		}, actualCollectors)
	})

	t.Run("disable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["query_sample", "schema_table"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QuerySampleName: false,
			collector.SchemaTableName: false,
		}, actualCollectors)
	})

	t.Run("enable collectors takes precedence over disable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["query_sample", "schema_table"]
		enable_collectors = ["query_sample", "schema_table"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QuerySampleName: true,
			collector.SchemaTableName: true,
		}, actualCollectors)
	})

	t.Run("enabling one and disabling one", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["schema_table"]
		enable_collectors = ["query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QuerySampleName: true,
			collector.SchemaTableName: false,
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
			collector.QuerySampleName: true,
			collector.SchemaTableName: true,
		}, actualCollectors)
	})
}
