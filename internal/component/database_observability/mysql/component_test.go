package mysql

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	http_service "github.com/grafana/alloy/internal/service/http"
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

type noopLogger struct{}

func (d *noopLogger) Log(_ ...interface{}) error {
	return nil
}

func Test_New(t *testing.T) {
	t.Run("all configurable collectors disabled leaves only 1 collector enabled", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = "root:secret_password@tcp(localhost:3306)/mydb"
		forward_to = []
		disable_collectors = ["query_sample", "schema_table"]
	`

		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args))

		db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		comp, err := New(
			component.Options{
				GetServiceData: func(name string) (interface{}, error) { return http_service.Data{}, nil },
				OnStateChange:  func(e component.Exports) {},
				Logger:         &noopLogger{},
			},
			args,
			db)
		require.NoError(t, err)

		assert.Len(t, comp.collectors, 1)
	})

	t.Run("default number of enabled collectors is 3", func(t *testing.T) {
		db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		var exampleDBO11yAlloyConfig = `
		data_source_name = "root:secret_password@tcp(localhost:3306)/mydb"
		forward_to = []
	`

		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args))

		comp, err := New(
			component.Options{
				GetServiceData: func(name string) (interface{}, error) { return http_service.Data{}, nil },
				OnStateChange:  func(e component.Exports) {},
				Logger:         &noopLogger{},
			},
			args,
			db)
		require.NoError(t, err)

		assert.Len(t, comp.collectors, 3)
	})
}
