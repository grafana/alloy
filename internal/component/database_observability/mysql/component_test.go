package mysql

import (
	"testing"

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
			querySample: true,
			schemaTable: true,
		}, actualCollectors)
	})

	t.Run("enable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		enable_collectors = ["QuerySample", "SchemaTable"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			querySample: true,
			schemaTable: true,
		}, actualCollectors)
	})

	t.Run("disable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["QuerySample", "SchemaTable"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			querySample: false,
			schemaTable: false,
		}, actualCollectors)
	})

	t.Run("enable collectors takes precedence over disable collectors", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["QuerySample", "SchemaTable"]
		enable_collectors = ["QuerySample", "SchemaTable"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			querySample: true,
			schemaTable: true,
		}, actualCollectors)
	})

	t.Run("enabling one and disabling one", func(t *testing.T) {
		var exampleDBO11yAlloyConfig = `
		data_source_name = ""
		forward_to = []
		disable_collectors = ["SchemaTable"]
		enable_collectors = ["QuerySample"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			querySample: true,
			schemaTable: false,
		}, actualCollectors)
	})
}
