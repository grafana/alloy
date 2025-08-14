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
			collector.QuerySampleName: false,
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
			collector.QuerySampleName: false,
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
			collector.QuerySampleName: false,
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
			collector.QuerySampleName: false,
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
			collector.QuerySampleName: false,
		}, actualCollectors)
	})

	t.Run("enable query_sample collector", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.QuerySampleName: true,
		}, actualCollectors)
	})

	t.Run("enable multiple collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["query_tables", "query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: true,
			collector.QuerySampleName: true,
		}, actualCollectors)
	})

	t.Run("disable query_sample collector", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		disable_collectors = ["query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryTablesName: false,
			collector.QuerySampleName: false,
		}, actualCollectors)
	})
}

func TestQueryRedactionConfig(t *testing.T) {
	t.Run("default behavior - query redaction enabled", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["query_sample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.False(t, args.DisableQueryRedaction, "query redaction should be enabled by default")
	})

	t.Run("explicitly disable query redaction", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["query_sample"]
		disable_query_redaction = true
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.True(t, args.DisableQueryRedaction, "query redaction should be disabled when explicitly set")
	})

	t.Run("explicitly enable query redaction", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		enable_collectors = ["query_sample"]
		disable_query_redaction = false
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.False(t, args.DisableQueryRedaction, "query redaction should be enabled when explicitly set to false")
	})
}
