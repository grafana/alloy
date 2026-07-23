package sql_server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability/sql_server/collector"
	"github.com/grafana/alloy/syntax"
)

func Test_parseCloudProvider(t *testing.T) {
	t.Run("parse aws cloud provider block", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to = []
		targets = []
		cloud_provider {
			aws {
				arn = "arn:aws:rds:some-region:some-account:db:some-db-instance"
			}
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		require.NotNil(t, args.CloudProvider)
		require.NotNil(t, args.CloudProvider.AWS)
		assert.Equal(t, "arn:aws:rds:some-region:some-account:db:some-db-instance", args.CloudProvider.AWS.ARN)
	})

	t.Run("parse azure cloud provider block with all fields", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to = []
		targets = []
		cloud_provider {
			azure {
				subscription_id = "sub-12345-abcde"
				resource_group  = "my-resource-group"
				server_name     = "my-sql-server"
			}
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		require.NotNil(t, args.CloudProvider)
		require.NotNil(t, args.CloudProvider.Azure)
		assert.Equal(t, "sub-12345-abcde", args.CloudProvider.Azure.SubscriptionID)
		assert.Equal(t, "my-resource-group", args.CloudProvider.Azure.ResourceGroup)
		assert.Equal(t, "my-sql-server", args.CloudProvider.Azure.ServerName)
	})

	t.Run("parse azure cloud provider block without optional server_name", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to = []
		targets = []
		cloud_provider {
			azure {
				subscription_id = "sub-12345-abcde"
				resource_group  = "my-resource-group"
			}
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		require.NotNil(t, args.CloudProvider)
		require.NotNil(t, args.CloudProvider.Azure)
		assert.Equal(t, "sub-12345-abcde", args.CloudProvider.Azure.SubscriptionID)
		assert.Equal(t, "my-resource-group", args.CloudProvider.Azure.ResourceGroup)
		assert.Empty(t, args.CloudProvider.Azure.ServerName)
	})

	t.Run("parse gcp cloud provider block", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to = []
		targets = []
		cloud_provider {
			gcp {
				connection_name = "my-gcp-project:us-central1:my-cloud-sql-instance"
			}
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		require.NotNil(t, args.CloudProvider)
		require.NotNil(t, args.CloudProvider.GCP)
		assert.Equal(t, "my-gcp-project:us-central1:my-cloud-sql-instance", args.CloudProvider.GCP.ConnectionName)
	})

	t.Run("empty cloud provider block", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to = []
		targets = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Nil(t, args.CloudProvider)
	})

	t.Run("multiple cloud providers returns error", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to = []
		targets = []
		cloud_provider {
			aws {
				arn = "arn:aws:rds:us-east-1:123456789012:db:mydb"
			}
			azure {
				subscription_id = "sub-12345-abcde"
				resource_group  = "my-resource-group"
			}
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.EqualError(t, err, "cloud_provider: at most one of aws, azure, or gcp must be specified")
	})
}

func TestQueryMetricsDefaults(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	assert.Equal(t, 1*time.Minute, args.QueryMetricsArguments.CollectInterval)
	assert.Equal(t, 50, args.QueryMetricsArguments.StatementsLimit)
	assert.Equal(t, 1*time.Hour, args.QueryMetricsArguments.StatementsLookback)
}

func TestQueryMetricsEnabledByDefault(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	collectors := enableOrDisableCollectors(args)
	assert.True(t, collectors[collector.QueryMetricsCollector])

	args.DisableCollectors = []string{collector.QueryMetricsCollector}
	collectors = enableOrDisableCollectors(args)
	assert.False(t, collectors[collector.QueryMetricsCollector])
}

func TestValidateQueryMetrics(t *testing.T) {
	base := func() Arguments {
		var args Arguments
		args.SetToDefault()
		args.DataSourceName = "sqlserver://user:pass@localhost:1433?database=app"
		return args
	}

	t.Run("defaults are valid", func(t *testing.T) {
		args := base()
		require.NoError(t, args.Validate())
	})

	t.Run("non-positive collect_interval is rejected", func(t *testing.T) {
		args := base()
		args.QueryMetricsArguments.CollectInterval = 0
		require.ErrorContains(t, args.Validate(), "query_metrics.collect_interval")
	})

	t.Run("non-positive statements_limit is rejected", func(t *testing.T) {
		args := base()
		args.QueryMetricsArguments.StatementsLimit = 0
		require.ErrorContains(t, args.Validate(), "query_metrics.statements_limit")
	})

	t.Run("non-positive statements_lookback is rejected", func(t *testing.T) {
		args := base()
		args.QueryMetricsArguments.StatementsLookback = 0
		require.ErrorContains(t, args.Validate(), "query_metrics.statements_lookback")
	})

	t.Run("invalid values are ignored when the collector is disabled", func(t *testing.T) {
		args := base()
		args.DisableCollectors = []string{collector.QueryMetricsCollector}
		args.QueryMetricsArguments.StatementsLimit = 0
		args.QueryMetricsArguments.StatementsLookback = 0
		require.NoError(t, args.Validate())
	})
}
