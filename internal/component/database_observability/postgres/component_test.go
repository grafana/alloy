package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	kitlog "github.com/go-kit/log"
	cmp "github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/loki/pkg/push"
)

func Test_enableOrDisableCollectors(t *testing.T) {
	t.Run("nothing specified (default behavior)", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("enable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["query_details"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		disable_collectors = ["query_details"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  false,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("enable collectors takes precedence over disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		disable_collectors = ["query_details"]
		enable_collectors = ["query_details"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("unknown collectors are ignored", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["some_string"]
		disable_collectors = ["another_string"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("enable query_samples collector", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["query_samples"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("enable schema_details collector", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["schema_details"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("enable multiple collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["query_details", "query_samples"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  true,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})

	t.Run("disable query_samples collector", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		disable_collectors = ["query_samples"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:  true,
			collector.QuerySamplesCollector:  false,
			collector.SchemaDetailsCollector: true,
			collector.ExplainPlanCollector:   true,
		}, actualCollectors)
	})
}

func TestQueryRedactionConfig(t *testing.T) {
	t.Run("default behavior - query redaction enabled", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["query_samples"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.False(t, args.QuerySampleArguments.DisableQueryRedaction, "query redaction should be enabled by default")
	})

	t.Run("explicitly disable query redaction", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["query_samples"]
		query_samples {
			disable_query_redaction = true
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.True(t, args.QuerySampleArguments.DisableQueryRedaction, "query redaction should be disabled when explicitly set")
	})

	t.Run("explicitly enable query redaction", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		enable_collectors = ["query_samples"]
		query_samples {
			disable_query_redaction = false
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.False(t, args.QuerySampleArguments.DisableQueryRedaction, "query redaction should be enabled when explicitly set to false")
	})
}

func TestCollectionIntervals(t *testing.T) {
	t.Run("default intervals", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.Equal(t, DefaultArguments.QuerySampleArguments.CollectInterval, args.QuerySampleArguments.CollectInterval, "collect_interval for query_samples should default to 15 seconds")
	})

	t.Run("custom intervals", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		query_samples {
			collect_interval = "5s"
		}
		`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, args.QuerySampleArguments.CollectInterval, "collect_interval for query_samples should be set to 5 seconds")
	})
}

func Test_addLokiLabels(t *testing.T) {
	t.Run("add required labels to loki entries", func(t *testing.T) {
		handler := loki.NewCollectingHandler()
		defer handler.Stop()
		entryHandler := addLokiLabels(handler, "some-instance-key", "some-system-id")

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
			"server_id": model.LabelValue("some-system-id"),
		}, handler.Received()[0].Labels)
		assert.Equal(t, "some-message", handler.Received()[0].Line)
	})
}

func TestPostgres_Update_DBUnavailable_ReportsUnhealthy(t *testing.T) {
	args := Arguments{DataSourceName: "postgres://127.0.0.1:1/db?sslmode=disable"}
	opts := cmp.Options{
		ID:            "test.postgres",
		Logger:        kitlog.NewNopLogger(),
		OnStateChange: func(e cmp.Exports) {},
		GetServiceData: func(name string) (any, error) {
			return http_service.Data{MemoryListenAddr: "127.0.0.1:0", BaseHTTPPath: "/component"}, nil
		},
	}
	c, err := New(opts, args)
	require.NoError(t, err)

	h := c.CurrentHealth()
	assert.Equal(t, cmp.HealthTypeUnhealthy, h.Health)
	assert.NotEmpty(t, h.Message)
}

func TestPostgres_schema_details_collect_interval_is_parsed_from_config(t *testing.T) {
	exampleDBO11yAlloyConfig := `
	data_source_name = "postgres://db"
	forward_to = []
	targets = []
	schema_details {
		collect_interval = "11s"
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
	require.NoError(t, err)

	assert.Equal(t, 11*time.Second, args.SchemaDetailsArguments.CollectInterval)
}

func TestPostgres_schema_details_cache_configuration_is_parsed_from_config(t *testing.T) {
	t.Run("default cache configuration", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Equal(t, DefaultArguments.SchemaDetailsArguments.CacheEnabled, args.SchemaDetailsArguments.CacheEnabled)
		assert.Equal(t, DefaultArguments.SchemaDetailsArguments.CacheSize, args.SchemaDetailsArguments.CacheSize)
		assert.Equal(t, DefaultArguments.SchemaDetailsArguments.CacheTTL, args.SchemaDetailsArguments.CacheTTL)
	})

	t.Run("custom cache configuration", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		schema_details {
			collect_interval = "30s"
			cache_enabled = false
			cache_size = 512
			cache_ttl = "5m"
		}
		`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Equal(t, 30*time.Second, args.SchemaDetailsArguments.CollectInterval)
		assert.False(t, args.SchemaDetailsArguments.CacheEnabled)
		assert.Equal(t, 512, args.SchemaDetailsArguments.CacheSize)
		assert.Equal(t, 5*time.Minute, args.SchemaDetailsArguments.CacheTTL)
	})
}

func Test_parseCloudProvider(t *testing.T) {
	t.Run("parse aws cloud provider block", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
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
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		cloud_provider {
			azure {
				subscription_id = "sub-12345-abcde"
				resource_group  = "my-resource-group"
				server_name     = "my-postgres-server"
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
		assert.Equal(t, "my-postgres-server", args.CloudProvider.Azure.ServerName)
	})

	t.Run("parse azure cloud provider block without optional server_name", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
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

	t.Run("empty cloud provider block", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Nil(t, args.CloudProvider)
	})
}

func Test_ErrorLogsCollector_StartsIndependentlyOfDatabase(t *testing.T) {
	t.Run("error_logs receiver is exported immediately on component creation", func(t *testing.T) {
		var exports Exports
		opts := cmp.Options{
			ID:         "test-component",
			Logger:     kitlog.NewNopLogger(),
			Registerer: nil,
			OnStateChange: func(e cmp.Exports) {
				exports = e.(Exports)
			},
			GetServiceData: func(name string) (any, error) {
				return http_service.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "",
					BaseHTTPPath:     "/",
					DialFunc:         nil,
				}, nil
			},
		}

		args := Arguments{
			DataSourceName: alloytypes.Secret("postgres://user:pass@localhost:5432/testdb"),
			ForwardTo:      []loki.LogsReceiver{loki.NewLogsReceiver()},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)
		require.NotNil(t, c)

		require.NotNil(t, exports.ErrorLogsReceiver, "ErrorLogsReceiver should be exported immediately")
		require.NotNil(t, c.errorLogsReceiver, "component should have errorLogsReceiver initialized")
		require.NotNil(t, c.errorLogsReceiver.Chan(), "receiver channel should be initialized")

		assert.Equal(t, c.errorLogsReceiver, exports.ErrorLogsReceiver,
			"exported receiver should be the same as component's internal receiver")
	})

	t.Run("collector field exists for runtime initialization", func(t *testing.T) {
		opts := cmp.Options{
			ID:            "test-component",
			Logger:        kitlog.NewNopLogger(),
			Registerer:    nil,
			OnStateChange: func(e cmp.Exports) {},
			GetServiceData: func(name string) (any, error) {
				return http_service.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "",
					BaseHTTPPath:     "/",
					DialFunc:         nil,
				}, nil
			},
		}

		args := Arguments{
			DataSourceName: alloytypes.Secret("postgres://user:pass@localhost:5432/testdb"),
			ForwardTo:      []loki.LogsReceiver{loki.NewLogsReceiver()},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		assert.Nil(t, c.errorLogsCollector, "errorLogsCollector should be nil before Run() is called")
	})
}

func Test_connectAndStartCollectors(t *testing.T) {
	t.Run("returns error when database connection fails", func(t *testing.T) {
		opts := cmp.Options{
			ID:            "test-component",
			Logger:        kitlog.NewNopLogger(),
			Registerer:    nil,
			OnStateChange: func(e cmp.Exports) {},
			GetServiceData: func(name string) (any, error) {
				return http_service.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "",
					BaseHTTPPath:     "/",
					DialFunc:         nil,
				}, nil
			},
		}

		args := Arguments{
			DataSourceName: alloytypes.Secret("postgres://user:pass@127.0.0.1:1/unreachable?sslmode=disable&connect_timeout=1"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		err = c.connectAndStartCollectors(context.Background())
		assert.Error(t, err, "should return error when connection fails")
		assert.Contains(t, err.Error(), "failed to", "error should indicate connection failure")
	})

	t.Run("closes existing connection before reconnecting", func(t *testing.T) {
		opts := cmp.Options{
			ID:            "test-component",
			Logger:        kitlog.NewNopLogger(),
			Registerer:    nil,
			OnStateChange: func(e cmp.Exports) {},
			GetServiceData: func(name string) (any, error) {
				return http_service.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "",
					BaseHTTPPath:     "/",
					DialFunc:         nil,
				}, nil
			},
		}

		args := Arguments{
			DataSourceName: alloytypes.Secret("postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		assert.Nil(t, c.dbConnection, "dbConnection should be nil initially after failed connection")

		err = c.connectAndStartCollectors(context.Background())
		assert.Error(t, err, "should return error for unreachable database")
	})
}

func TestPostgres_Reconnection(t *testing.T) {
	t.Run("tryReconnect fails and maintains health error", func(t *testing.T) {
		opts := cmp.Options{
			ID:            "test",
			Logger:        kitlog.NewNopLogger(),
			OnStateChange: func(e cmp.Exports) {},
			GetServiceData: func(name string) (any, error) {
				return http_service.Data{MemoryListenAddr: "127.0.0.1:0", BaseHTTPPath: "/"}, nil
			},
		}

		args := Arguments{
			DataSourceName: alloytypes.Secret("postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		c.healthErr.Store("initial error")

		err = c.tryReconnect(context.Background())
		assert.Error(t, err)
		assert.NotEmpty(t, c.healthErr.Load())
	})

	t.Run("tryReconnect succeeds and clears health error", func(t *testing.T) {
		opts := cmp.Options{
			ID:            "test",
			Logger:        kitlog.NewNopLogger(),
			OnStateChange: func(e cmp.Exports) {},
			GetServiceData: func(name string) (any, error) {
				return http_service.Data{MemoryListenAddr: "127.0.0.1:0", BaseHTTPPath: "/"}, nil
			},
		}

		args := Arguments{
			DataSourceName:    alloytypes.Secret("postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"),
			ForwardTo:         []loki.LogsReceiver{},
			Targets:           []discovery.Target{},
			DisableCollectors: []string{"query_details", "schema_details", "query_samples", "explain_plans"},
			HealthCheckArguments: HealthCheckArguments{
				CollectInterval: 1 * time.Hour,
			},
		}

		// First mock: will fail
		db1, mock1, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db1.Close()

		mock1.ExpectPing().WillReturnError(assert.AnError)

		c := &Component{
			opts:      opts,
			args:      args,
			receivers: args.ForwardTo,
			handler:   loki.NewLogsReceiver(),
			registry:  prometheus.NewRegistry(),
			healthErr: atomic.NewString(""),
			openSQL:   func(_ string, _ string) (*sql.DB, error) { return db1, nil },
		}
		c.instanceKey = "test-instance"
		c.baseTarget = discovery.NewTargetFromMap(map[string]string{
			"instance": c.instanceKey,
			"job":      "database_observability",
		})

		// First attempt: connection fails
		err = c.tryReconnect(context.Background())
		assert.Error(t, err)
		assert.NotEmpty(t, c.healthErr.Load())

		// Second mock: will succeed
		db2, mock2, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db2.Close()

		mock2.ExpectPing()
		mock2.ExpectQuery(`SELECT.*system_identifier.*inet_server_addr.*inet_server_port.*version`).
			WillReturnRows(sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
				AddRow("1234567890", "127.0.0.1", "5432", "14.0"))

		c.openSQL = func(_ string, _ string) (*sql.DB, error) { return db2, nil }

		// Second attempt: connection succeeds and clears error
		err = c.tryReconnect(context.Background())
		assert.NoError(t, err)
		assert.Empty(t, c.healthErr.Load())
	})

	t.Run("Run exits on context cancellation", func(t *testing.T) {
		opts := cmp.Options{
			ID:            "test",
			Logger:        kitlog.NewNopLogger(),
			OnStateChange: func(e cmp.Exports) {},
			GetServiceData: func(name string) (any, error) {
				return http_service.Data{MemoryListenAddr: "127.0.0.1:0", BaseHTTPPath: "/"}, nil
			},
		}

		args := Arguments{
			DataSourceName:    alloytypes.Secret("postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"),
			ForwardTo:         []loki.LogsReceiver{},
			Targets:           []discovery.Target{},
			DisableCollectors: []string{"query_details", "schema_details", "query_samples", "explain_plans"},
			HealthCheckArguments: HealthCheckArguments{
				CollectInterval: 1 * time.Hour,
			},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		runErr := make(chan error, 1)
		go func() {
			runErr <- c.Run(ctx)
		}()

		time.Sleep(100 * time.Millisecond)
		cancel()

		select {
		case err := <-runErr:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not exit after context cancellation")
		}
=======
		// Before Run(), errorLogsCollector should be nil
		require.Nil(t, c.errorLogsCollector, "collector should be nil before Run()")

		// In Run(), the collector gets created before DB connection attempt.
		// Unit tests in error_logs_test.go validate:
		// - Collectors work without any DB connection
		// - SystemID can be updated dynamically
		// - Logs are processed with empty systemID initially
>>>>>>> c7618237b (feat(postgres): add error_logs collector)
	})
}
