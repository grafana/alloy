package sql_server

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/sql_server/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/syntax"
)

func Test_addLokiLabels(t *testing.T) {
	t.Run("add required labels to loki entries", func(t *testing.T) {
		handler := loki.NewCollectingHandler()
		defer handler.Stop()
		entryHandler := addLokiLabels(handler, "some-instance-key", "some-server-id-hash")

		go func() {
			ts := time.Now().UnixNano()
			entryHandler.Chan() <- loki.Entry{
				Entry: push.Entry{
					Timestamp: time.Unix(0, ts),
					Line:      "some-message",
				},
			}
		}()

		require.Eventually(t, func() bool {
			return len(handler.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		require.Len(t, handler.Received(), 1)
		assert.Equal(t, model.LabelSet{
			"job":       database_observability.JobName,
			"instance":  model.LabelValue("some-instance-key"),
			"server_id": model.LabelValue("some-server-id-hash"),
			"engine":    model.LabelValue(collector.EngineName),
		}, handler.Received()[0].Labels)
		assert.Equal(t, "some-message", handler.Received()[0].Line)
	})
}

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

func TestQueryTimeout(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		var args Arguments
		args.SetToDefault()

		assert.Equal(t, 10*time.Second, args.QueryTimeout)
	})

	t.Run("configuration is parsed", func(t *testing.T) {
		config := `
			data_source_name = "sqlserver://user:pass@localhost:1433"
			forward_to       = []
			query_timeout    = "15s"
		`

		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(config), &args))
		assert.Equal(t, 15*time.Second, args.QueryTimeout)
	})

	t.Run("non-positive value is rejected", func(t *testing.T) {
		var args Arguments
		args.SetToDefault()
		args.DataSourceName = "sqlserver://user:pass@localhost:1433?database=app"
		args.QueryTimeout = 0
		require.EqualError(t, args.Validate(), "query_timeout must be greater than zero")
	})
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

func TestExplainPlansDefaults(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	assert.Equal(t, 1*time.Minute, args.ExplainPlansArguments.CollectInterval)
}

func TestExplainPlansEnabledByDefault(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	collectors := enableOrDisableCollectors(args)
	assert.True(t, collectors[collector.ExplainPlansCollector])

	args.DisableCollectors = []string{collector.ExplainPlansCollector}
	collectors = enableOrDisableCollectors(args)
	assert.False(t, collectors[collector.ExplainPlansCollector])
}

func TestValidateExplainPlans(t *testing.T) {
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
		args.ExplainPlansArguments.CollectInterval = 0
		require.ErrorContains(t, args.Validate(), "explain_plans.collect_interval")
	})

	t.Run("invalid values are ignored when the collector is disabled", func(t *testing.T) {
		args := base()
		args.DisableCollectors = []string{collector.ExplainPlansCollector}
		args.ExplainPlansArguments.CollectInterval = 0
		require.NoError(t, args.Validate())
	})
}

func TestExplainPlansConfigParsing(t *testing.T) {
	exampleDBO11yAlloyConfig := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to       = []
		targets          = []
		explain_plans {
			collect_interval = "5m"
		}
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, args.ExplainPlansArguments.CollectInterval)
}

func TestQuerySamplesDefaults(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	assert.Equal(t, 10*time.Second, args.QuerySamplesArguments.CollectInterval)
	assert.False(t, args.QuerySamplesArguments.DisableQueryRedaction)
	assert.True(t, args.ExcludeCurrentUser)
	assert.Equal(t, []string{
		"azuresu",
		"cloudsqladmin",
		"db-o11y",
		"rdsadmin",
	}, args.ExcludeUsers)
}

func TestQuerySamplesEnabledByDefault(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	collectors := enableOrDisableCollectors(args)
	assert.True(t, collectors[collector.QuerySamplesCollector])

	args.DisableCollectors = []string{collector.QuerySamplesCollector}
	collectors = enableOrDisableCollectors(args)
	assert.False(t, collectors[collector.QuerySamplesCollector])
}

func TestValidateQuerySamples(t *testing.T) {
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
		args.QuerySamplesArguments.CollectInterval = 0
		require.ErrorContains(t, args.Validate(), "query_samples.collect_interval")
	})

	t.Run("invalid values are ignored when disabled", func(t *testing.T) {
		args := base()
		args.DisableCollectors = []string{collector.QuerySamplesCollector}
		args.QuerySamplesArguments.CollectInterval = 0
		require.NoError(t, args.Validate())
	})
}

func TestQuerySamplesConfigParsing(t *testing.T) {
	config := `
		data_source_name    = "sqlserver://user:pass@localhost:1433"
		forward_to          = []
		exclude_users       = ["app_reader", "batch_user"]
		exclude_current_user = false
		query_samples {
			collect_interval        = "5s"
			disable_query_redaction = true
		}
	`

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(config), &args))
	assert.Equal(t, 5*time.Second, args.QuerySamplesArguments.CollectInterval)
	assert.True(t, args.QuerySamplesArguments.DisableQueryRedaction)
	assert.Equal(t, []string{"app_reader", "batch_user"}, args.ExcludeUsers)
	assert.False(t, args.ExcludeCurrentUser)
}

func TestResolveExcludeUsers(t *testing.T) {
	t.Run("merges original login without mutating configured users", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		configured := []string{"app_reader"}
		mock.ExpectQuery(selectOriginalLogin).
			WillReturnRows(sqlmock.NewRows([]string{"original_login"}).AddRow("alloy_monitor"))

		effective, err := resolveExcludeUsers(context.Background(), db, defaultQueryTimeout, configured, true)
		require.NoError(t, err)
		assert.Equal(t, []string{"app_reader", "alloy_monitor"}, effective)
		assert.Equal(t, []string{"app_reader"}, configured)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("preserves differently cased login names", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectOriginalLogin).
			WillReturnRows(sqlmock.NewRows([]string{"original_login"}).AddRow("Alloy_Monitor"))

		effective, err := resolveExcludeUsers(context.Background(), db, defaultQueryTimeout, []string{"alloy_monitor"}, true)
		require.NoError(t, err)
		assert.Equal(t, []string{"alloy_monitor", "Alloy_Monitor"}, effective)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("disabled skips original login query", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		effective, err := resolveExcludeUsers(context.Background(), db, defaultQueryTimeout, []string{"app_reader"}, false)
		require.NoError(t, err)
		assert.Equal(t, []string{"app_reader"}, effective)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query failure is wrapped", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectOriginalLogin).WillReturnError(errors.New("permission denied"))
		_, err = resolveExcludeUsers(context.Background(), db, defaultQueryTimeout, nil, true)
		require.ErrorContains(t, err, "failed to query original login")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestConnectAndStartCollectorsFailsWhenCurrentUserCannotBeResolved(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectPing()
	mock.ExpectQuery(selectServerInfo).
		WillReturnRows(sqlmock.NewRows([]string{"server_name", "machine_name", "product_version"}).
			AddRow("server", "machine", "16.0"))
	mock.ExpectQuery(selectOriginalLogin).WillReturnError(errors.New("permission denied"))

	var args Arguments
	args.SetToDefault()
	args.ExcludeCurrentUser = true
	c := &Component{
		args: args,
		openSQL: func(_, _ string) (*sql.DB, error) {
			return db, nil
		},
	}
	inst := &dbInstance{}
	c.storeInstances([]*dbInstance{inst})

	err = c.connectAndStartCollectors(context.Background(), inst)
	require.ErrorContains(t, err, "failed to resolve current login for query_samples user exclusion")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateDatabaseInstance(t *testing.T) {
	t.Run("multiple database_instance blocks with distinct servers is valid", func(t *testing.T) {
		cfg := `
		forward_to = []
		database_instance "one" {
			data_source_name = "sqlserver://user:pass@host-one:1433?database=db1"
		}
		database_instance "two" {
			data_source_name = "sqlserver://user:pass@host-two:1433?database=db2"
		}
		`
		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
		require.NoError(t, args.Validate())
		require.Len(t, args.Databases, 2)
	})

	t.Run("legacy single-DSN form without database_instance blocks is still valid", func(t *testing.T) {
		cfg := `
		data_source_name = "sqlserver://user:pass@localhost:1433"
		forward_to = []
		targets = []
		`
		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
		require.NoError(t, args.Validate())
	})

	t.Run("top-level data_source_name and database_instance blocks are mutually exclusive", func(t *testing.T) {
		args := Arguments{
			DataSourceName: "sqlserver://user:pass@host-one:1433?database=db1",
			Databases: []DatabaseArguments{
				{Name: "one", DataSourceName: "sqlserver://user:pass@host-one:1433?database=db1"},
			},
		}
		require.ErrorContains(t, args.Validate(), "data_source_name and database_instance blocks are mutually exclusive")
	})

	t.Run("top-level targets and database_instance blocks are mutually exclusive", func(t *testing.T) {
		args := Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"foo": "bar"})},
			Databases: []DatabaseArguments{
				{Name: "one", DataSourceName: "sqlserver://user:pass@host-one:1433?database=db1"},
			},
		}
		require.ErrorContains(t, args.Validate(), "targets and database_instance blocks are mutually exclusive")
	})

	t.Run("top-level cloud_provider and database_instance blocks are mutually exclusive", func(t *testing.T) {
		args := Arguments{
			CloudProvider: &CloudProvider{AWS: &AWSCloudProviderInfo{ARN: "some-arn"}},
			Databases: []DatabaseArguments{
				{Name: "one", DataSourceName: "sqlserver://user:pass@host-one:1433?database=db1"},
			},
		}
		require.ErrorContains(t, args.Validate(), "cloud_provider and database_instance blocks are mutually exclusive")
	})

	t.Run("duplicate database_instance labels are rejected", func(t *testing.T) {
		args := Arguments{
			Databases: []DatabaseArguments{
				{Name: "one", DataSourceName: "sqlserver://user:pass@host-one:1433?database=db1"},
				{Name: "one", DataSourceName: "sqlserver://user:pass@host-two:1433?database=db2"},
			},
		}
		require.ErrorContains(t, args.Validate(), `duplicate database_instance block label "one"`)
	})

	t.Run("database_instance blocks resolving to the same server are rejected", func(t *testing.T) {
		args := Arguments{
			Databases: []DatabaseArguments{
				{Name: "one", DataSourceName: "sqlserver://user:pass@same-host:1433?database=same-db"},
				{Name: "two", DataSourceName: "sqlserver://user:pass@same-host:1433?database=same-db"},
			},
		}
		require.ErrorContains(t, args.Validate(), `resolve to the same server`)
	})

	t.Run("invalid database_instance label is rejected", func(t *testing.T) {
		args := Arguments{
			Databases: []DatabaseArguments{
				{Name: "1-invalid", DataSourceName: "sqlserver://user:pass@host-one:1433?database=db1"},
			},
		}
		require.ErrorContains(t, args.Validate(), "must be a valid identifier")
	})
}
