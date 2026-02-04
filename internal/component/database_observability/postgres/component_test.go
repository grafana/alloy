package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

		// Use unreachable DSN to trigger connection error
		args := Arguments{
			DataSourceName: alloytypes.Secret("postgres://user:pass@127.0.0.1:1/unreachable?sslmode=disable&connect_timeout=1"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		// Verify that connectAndStartCollectors returns an error
		err = c.connectAndStartCollectors(context.Background())
		assert.Error(t, err, "should return error when connection fails")
		assert.Contains(t, err.Error(), "failed to", "error should indicate connection failure")
	})

	t.Run("closes existing connection before reconnecting", func(t *testing.T) {
		// This test verifies that connectAndStartCollectors properly closes
		// an existing connection before attempting a new one

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

		// The component should handle nil dbConnection gracefully
		assert.Nil(t, c.dbConnection, "dbConnection should be nil initially after failed connection")

		// Calling connectAndStartCollectors again should not panic
		err = c.connectAndStartCollectors(context.Background())
		assert.Error(t, err, "should return error for unreachable database")
	})
}

func Test_tryReconnect(t *testing.T) {
	t.Run("clears health error on successful reconnection", func(t *testing.T) {
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

		// Set a health error
		c.healthErr.Store("initial error")

		// Try to reconnect (will fail with unreachable database)
		err = c.tryReconnect(context.Background())
		assert.Error(t, err, "tryReconnect should return error when connection fails")

		// Health error should still be set since reconnection failed
		assert.NotEmpty(t, c.healthErr.Load(), "health error should remain when reconnection fails")
	})

	t.Run("returns error when connectAndStartCollectors fails", func(t *testing.T) {
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
			DataSourceName: alloytypes.Secret("postgres://invalid-dsn"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		err = c.tryReconnect(context.Background())
		assert.Error(t, err, "tryReconnect should propagate connection errors")
	})
}

func Test_automaticReconnection(t *testing.T) {
	t.Run("component reports unhealthy when database is unreachable", func(t *testing.T) {
		opts := cmp.Options{
			ID:            "test-health",
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

		// Use unreachable DSN
		args := Arguments{
			DataSourceName: alloytypes.Secret("postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		// Component should be unhealthy after failed connection
		health := c.CurrentHealth()
		assert.Equal(t, cmp.HealthTypeUnhealthy, health.Health, "component should be unhealthy when DB is unreachable")
		assert.Contains(t, health.Message, "failed to connect", "health message should indicate connection failure")
	})

	t.Run("reconnection ticker respects context cancellation", func(t *testing.T) {
		// This test verifies that the reconnection goroutine exits when context is cancelled
		opts := cmp.Options{
			ID:            "test-ctx",
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

		ctx, cancel := context.WithCancel(context.Background())

		// Start Run in a goroutine
		runErr := make(chan error, 1)
		go func() {
			runErr <- c.Run(ctx)
		}()

		// Give it a moment to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context
		cancel()

		// Run should exit quickly after cancellation
		select {
		case err := <-runErr:
			assert.NoError(t, err, "Run should exit cleanly")
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not exit after context cancellation")
		}
	})
}
