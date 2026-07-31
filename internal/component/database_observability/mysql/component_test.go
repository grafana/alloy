package mysql

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	cmp "github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	"github.com/grafana/alloy/internal/component/discovery"
	exporter_mysql "github.com/grafana/alloy/internal/component/prometheus/exporter/mysql"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/loki/pkg/push"
)

// testGetServiceData stubs the services the component requests: HTTP service
// data and a single-node mock cluster.
func testGetServiceData(name string) (any, error) {
	switch name {
	case cluster.ServiceName:
		return cluster.Mock(), nil
	default:
		return http_service.Data{MemoryListenAddr: "127.0.0.1:0", BaseHTTPPath: "/component"}, nil
	}
}

func Test_defaultExclusions(t *testing.T) {
	exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"alloydbadmin",
		"alloydbmetadata",
		"azure_maintenance",
		"azure_sys",
		"cloudsqladmin",
		"rdsadmin",
	}, args.ExcludeSchemas)
}

func Test_disableQueryRedaction(t *testing.T) {
	t.Run("enable sql text when provided", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
		query_samples {
			disable_query_redaction = true
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.True(t, args.QuerySamplesArguments.DisableQueryRedaction)
	})

	t.Run("disable sql text when not provided (default behavior)", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.False(t, args.QuerySamplesArguments.DisableQueryRedaction)
	})

	t.Run("setup consumers scrape interval is correctly parsed from config", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
		setup_consumers {
			collect_interval = "1h"
		}
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		assert.Equal(t, time.Hour, args.SetupConsumersArguments.CollectInterval)
	})
}

func Test_parseCloudProvider(t *testing.T) {
	t.Run("parse cloud provider block", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
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

		assert.Equal(t, "arn:aws:rds:some-region:some-account:db:some-db-instance", args.CloudProvider.AWS.ARN)
	})

	t.Run("parse azure cloud provider block with all fields", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
		cloud_provider {
			azure {
				subscription_id = "sub-12345-abcde"
				resource_group  = "my-resource-group"
				server_name     = "my-mysql-server"
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
		assert.Equal(t, "my-mysql-server", args.CloudProvider.Azure.ServerName)
	})

	t.Run("parse azure cloud provider block without optional server_name", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
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
		data_source_name = ""
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
		data_source_name = ""
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
		data_source_name = ""
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

func Test_enableOrDisableCollectors(t *testing.T) {
	t.Run("nothing specified (default behavior)", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:   true,
			collector.SchemaDetailsCollector:  true,
			collector.QuerySamplesCollector:   true,
			collector.SetupConsumersCollector: true,
			collector.SetupActorsCollector:    true,
			collector.ExplainPlansCollector:   true,
			collector.LocksCollector:          false,
		}, actualCollectors)
	})

	t.Run("enable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
		enable_collectors = ["query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:   true,
			collector.SchemaDetailsCollector:  true,
			collector.QuerySamplesCollector:   true,
			collector.SetupConsumersCollector: true,
			collector.SetupActorsCollector:    true,
			collector.ExplainPlansCollector:   true,
			collector.LocksCollector:          true,
		}, actualCollectors)
	})

	t.Run("disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
		disable_collectors = ["query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:   false,
			collector.SchemaDetailsCollector:  false,
			collector.QuerySamplesCollector:   false,
			collector.SetupConsumersCollector: false,
			collector.SetupActorsCollector:    false,
			collector.ExplainPlansCollector:   false,
			collector.LocksCollector:          false,
		}, actualCollectors)
	})

	t.Run("enable collectors takes precedence over disable collectors", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
		disable_collectors = ["query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"]
		enable_collectors = ["query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:   true,
			collector.SchemaDetailsCollector:  true,
			collector.QuerySamplesCollector:   true,
			collector.SetupConsumersCollector: true,
			collector.SetupActorsCollector:    true,
			collector.ExplainPlansCollector:   true,
			collector.LocksCollector:          true,
		}, actualCollectors)
	})

	t.Run("enabling one and disabling others", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
		forward_to = []
		targets = []
		disable_collectors = ["schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"]
		enable_collectors = ["query_details"]
	`

		var args Arguments
		err := syntax.Unmarshal([]byte(exampleDBO11yAlloyConfig), &args)
		require.NoError(t, err)

		actualCollectors := enableOrDisableCollectors(args)

		assert.Equal(t, map[string]bool{
			collector.QueryDetailsCollector:   true,
			collector.SchemaDetailsCollector:  false,
			collector.QuerySamplesCollector:   false,
			collector.SetupConsumersCollector: false,
			collector.SetupActorsCollector:    false,
			collector.ExplainPlansCollector:   false,
			collector.LocksCollector:          false,
		}, actualCollectors)
	})

	t.Run("unknown collectors are ignored", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = ""
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
			collector.QueryDetailsCollector:   true,
			collector.SchemaDetailsCollector:  true,
			collector.QuerySamplesCollector:   true,
			collector.SetupConsumersCollector: true,
			collector.SetupActorsCollector:    true,
			collector.ExplainPlansCollector:   true,
			collector.LocksCollector:          false,
		}, actualCollectors)
	})
}

func Test_addLokiLabels(t *testing.T) {
	t.Run("add required labels to loki entries", func(t *testing.T) {
		lokiClient := loki.NewCollectingHandler()
		defer lokiClient.Stop()
		entryHandler := addLokiLabels(lokiClient, "some-instance-key", "some-server-id-hash")

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
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		require.Len(t, lokiClient.Received(), 1)
		assert.Equal(t, model.LabelSet{
			"job":       database_observability.JobName,
			"instance":  model.LabelValue("some-instance-key"),
			"server_id": model.LabelValue("some-server-id-hash"),
		}, lokiClient.Received()[0].Labels)
		assert.Equal(t, "some-message", lokiClient.Received()[0].Line)
	})
}

// TestMySQL_Update_DBUnavailable_ReportsUnhealthy tests that the component does not return an error when the database is unavailable,
// but reports unhealthy with the error message from the database.
func TestMySQL_Update_DBUnavailable_ReportsUnhealthy(t *testing.T) {
	args := Arguments{DataSourceName: "user:pass@tcp(127.0.0.1:1)/db"}
	opts := cmp.Options{
		ID:             "test.mysql",
		Logger:         logging.NewSlogNop(),
		GetServiceData: testGetServiceData,
		OnStateChange:  func(e cmp.Exports) {},
	}
	c, err := New(opts, args)
	require.NoError(t, err)
	h := c.CurrentHealth()
	assert.Equal(t, cmp.HealthTypeUnhealthy, h.Health)
	assert.NotEmpty(t, h.Message)
}

// TestMySQL_StartCollectors_ReportsUnhealthy_StackedErrors tests that the component tries to start collectors on a best effort basis,
// reports unhealthy stacking errors for the collectors that failed to start and generate metrics for the collectors that started successfully.
func TestMySQL_StartCollectors_ReportsUnhealthy_StackedErrors(t *testing.T) {
	args := Arguments{
		DataSourceName:    "user:pass@tcp(127.0.0.1:3306)/db",
		DisableCollectors: []string{"query_details", "schema_details", "setup_consumers", "setup_actors", "explain_plans"},
		EnableCollectors:  []string{"query_samples", "locks"},
		QuerySamplesArguments: QuerySamplesArguments{
			CollectInterval:       time.Second,
			DisableQueryRedaction: true,
		},
		LocksArguments: LocksArguments{
			CollectInterval: time.Second,
			Threshold:       time.Second,
		},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}
	var gotExports cmp.Exports
	opts := cmp.Options{
		ID:             "test.mysql",
		Logger:         logging.NewSlogNop(),
		GetServiceData: testGetServiceData,
		OnStateChange:  func(e cmp.Exports) { gotExports = e },
	}

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	// First ping to the database succeeds, so we can start collectors
	mock.ExpectPing()
	// Engine info succeeds (if reached)
	mock.ExpectQuery(`SELECT @@server_uuid, @@hostname, VERSION\(\)`).WillReturnRows(sqlmock.NewRows([]string{"server_uuid", "hostname", "version"}).AddRow("uuid-1", "test-hostname", "8.0.0"))
	// QuerySample constructor queries uptime and fails
	mock.ExpectQuery(regexp.QuoteMeta("SELECT variable_value FROM performance_schema.global_status WHERE variable_name = 'UPTIME'")).
		WillReturnRows(sqlmock.NewRows([]string{"variable_value"}).AddRow(1))
	// Locks constructor Ping fails
	mock.ExpectPing().WillReturnError(assert.AnError)
	// Locks constructor Ping succeeds
	mock.ExpectPing()

	c, err := new(opts, args, func(_ string, _ string) (*sql.DB, error) { return db, nil })
	require.NoError(t, err)

	h := c.CurrentHealth()
	assert.Equal(t, cmp.HealthTypeUnhealthy, h.Health)
	assert.Contains(t, h.Message, collector.LocksCollector)

	exported, ok := gotExports.(Exports)
	require.True(t, ok)
	require.NotEmpty(t, exported.Targets)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	c.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	// connection_info remains 1 with labels
	assert.Regexp(t, `(?m)^database_observability_connection_info\{[^}]*engine=\"mysql\"[^}]*engine_version=\"8\.0\.0\"[^}]*\}\s+1(\.0+)?$`, body)
}

func TestMySQL_Reconnection(t *testing.T) {
	t.Run("tryReconnect fails and maintains health error", func(t *testing.T) {
		opts := cmp.Options{
			ID:             "test",
			Logger:         logging.NewSlogNop(),
			OnStateChange:  func(e cmp.Exports) {},
			GetServiceData: testGetServiceData,
		}

		args := Arguments{
			DataSourceName: alloytypes.Secret("user:pass@tcp(127.0.0.1:1)/db?timeout=100ms"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		c.loadInstances()[0].healthErr.Store("initial error")

		err = c.tryReconnect(context.Background())
		assert.Error(t, err)
		assert.NotEmpty(t, c.loadInstances()[0].healthErr.Load())
	})

	t.Run("tryReconnect succeeds and clears health error", func(t *testing.T) {
		opts := cmp.Options{
			ID:             "test",
			Logger:         logging.NewSlogNop(),
			OnStateChange:  func(e cmp.Exports) {},
			GetServiceData: testGetServiceData,
		}

		args := Arguments{
			DataSourceName:    alloytypes.Secret("user:pass@tcp(127.0.0.1:3306)/db"),
			ForwardTo:         []loki.LogsReceiver{},
			Targets:           []discovery.Target{},
			DisableCollectors: []string{"query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"},
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
			opts:    opts,
			args:    args,
			fanout:  loki.NewFanout(args.ForwardTo),
			handler: loki.NewLogsReceiver(),
			openSQL: func(_ string, _ string) (*sql.DB, error) { return db1, nil },
		}
		c.storeInstances([]*dbInstance{{
			cfg:         databaseConfig{dsn: args.DataSourceName},
			instanceKey: "test-instance",
			baseTarget: discovery.NewTargetFromMap(map[string]string{
				"instance": "test-instance",
				"job":      "database_observability",
			}),
			registry:  prometheus.NewRegistry(),
			healthErr: atomic.NewString(""),
		}})

		// First attempt: connection fails
		err = c.tryReconnect(context.Background())
		assert.Error(t, err)
		assert.NotEmpty(t, c.loadInstances()[0].healthErr.Load())

		// Second mock: will succeed
		db2, mock2, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db2.Close()

		mock2.ExpectPing()
		mock2.ExpectQuery(`SELECT @@server_uuid, @@hostname, VERSION\(\)`).
			WillReturnRows(sqlmock.NewRows([]string{"server_uuid", "hostname", "version"}).
				AddRow("uuid-1", "host-1", "8.0.0"))
		mock2.ExpectPing()

		c.openSQL = func(_ string, _ string) (*sql.DB, error) { return db2, nil }

		// Second attempt: connection succeeds and clears error
		err = c.tryReconnect(context.Background())
		assert.NoError(t, err)
		assert.Empty(t, c.loadInstances()[0].healthErr.Load())
	})

	t.Run("Run exits on context cancellation", func(t *testing.T) {
		opts := cmp.Options{
			ID:             "test",
			Logger:         logging.NewSlogNop(),
			OnStateChange:  func(e cmp.Exports) {},
			GetServiceData: testGetServiceData,
		}

		args := Arguments{
			DataSourceName: alloytypes.Secret("user:pass@tcp(127.0.0.1:1)/db?timeout=100ms"),
			ForwardTo:      []loki.LogsReceiver{},
			Targets:        []discovery.Target{},
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
	})
}

func Test_PrometheusExporterBlock(t *testing.T) {
	t.Run("absent when not specified", func(t *testing.T) {
		cfg := `
			data_source_name = ""
			forward_to = []
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.NoError(t, err)
		assert.Nil(t, args.PrometheusExporter)
	})

	t.Run("present with defaults when empty block", func(t *testing.T) {
		cfg := `
			data_source_name = ""
			forward_to = []
			prometheus_exporter {}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.NoError(t, err)
		require.NotNil(t, args.PrometheusExporter)
		exporterArgs := exporter_mysql.Arguments(*args.PrometheusExporter)
		assert.Equal(t, 2, exporterArgs.LockWaitTimeout) // default value
	})

	t.Run("present with custom collectors", func(t *testing.T) {
		cfg := `
			data_source_name = ""
			forward_to = []
			prometheus_exporter {
			  enable_collectors = ["perf_schema.eventsstatements", "perf_schema.eventswaits"]
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.NoError(t, err)
		require.NotNil(t, args.PrometheusExporter)
		exporterArgs := exporter_mysql.Arguments(*args.PrometheusExporter)
		assert.Equal(t, 2, exporterArgs.LockWaitTimeout) // default value
		assert.Equal(t, []string{"perf_schema.eventsstatements", "perf_schema.eventswaits"}, args.PrometheusExporter.EnableCollectors)
	})

	t.Run("error when both prometheus_exporter and targets are set", func(t *testing.T) {
		cfg := `
			data_source_name = ""
			forward_to = []
			targets = [{"__address__" = "localhost:9104"}]
			prometheus_exporter {}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, "prometheus_exporter and targets are mutually exclusive")
	})
}

func Test_databaseInstanceBlocks(t *testing.T) {
	t.Run("parse database blocks", func(t *testing.T) {
		cfg := `
			forward_to = []
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/orders"
				cloud_provider {
					aws {
						arn = "arn:aws:rds:us-east-1:123456789012:db:orders"
					}
				}
			}
			database_instance "billing" {
				data_source_name = "user:pass@tcp(localhost:3306)/billing"
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.NoError(t, err)

		require.Len(t, args.Databases, 2)
		assert.Equal(t, "orders", args.Databases[0].Name)
		assert.Equal(t, alloytypes.Secret("user:pass@tcp(localhost:3306)/orders"), args.Databases[0].DataSourceName)
		require.NotNil(t, args.Databases[0].CloudProvider)
		assert.Equal(t, "arn:aws:rds:us-east-1:123456789012:db:orders", args.Databases[0].CloudProvider.AWS.ARN)
		assert.Equal(t, "billing", args.Databases[1].Name)
	})

	t.Run("data_source_name and database_instance blocks are mutually exclusive", func(t *testing.T) {
		cfg := `
			data_source_name = "user:pass@tcp(localhost:3306)/db"
			forward_to = []
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/orders"
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, "data_source_name and database_instance blocks are mutually exclusive")
	})

	t.Run("targets and database_instance blocks are mutually exclusive", func(t *testing.T) {
		cfg := `
			forward_to = []
			targets = [{"__address__" = "localhost:9104"}]
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/orders"
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, "targets and database_instance blocks are mutually exclusive")
	})

	t.Run("cloud_provider and database_instance blocks are mutually exclusive", func(t *testing.T) {
		cfg := `
			forward_to = []
			cloud_provider {
				aws {
					arn = "arn:aws:rds:us-east-1:123456789012:db:mydb"
				}
			}
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/orders"
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, "cloud_provider and database_instance blocks are mutually exclusive")
	})

	t.Run("duplicate database_instance block labels", func(t *testing.T) {
		cfg := `
			forward_to = []
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/orders"
			}
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/billing"
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, `duplicate database_instance block label "orders"`)
	})

	t.Run("invalid database block label", func(t *testing.T) {
		// The syntax parser rejects non-identifier labels itself; the Validate
		// check is a backstop for programmatically constructed Arguments.
		args := Arguments{
			Databases: []DatabaseArguments{
				{Name: "bad name", DataSourceName: "user:pass@tcp(localhost:3306)/orders"},
			},
		}
		err := args.Validate()
		require.ErrorContains(t, err, "must be a valid identifier")
	})

	t.Run("database blocks resolving to the same server", func(t *testing.T) {
		cfg := `
			forward_to = []
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/orders"
			}
			database_instance "orders_replica" {
				data_source_name = "other:pass@tcp(localhost:3306)/orders"
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, "resolve to the same server")
	})

	t.Run("targets is not a valid attribute of database_instance blocks", func(t *testing.T) {
		cfg := `
			forward_to = []
			database_instance "orders" {
				data_source_name = "user:pass@tcp(localhost:3306)/orders"
				targets = [{"__address__" = "localhost:9104"}]
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, `unrecognized attribute name "targets"`)
	})
}

func multiDatabaseTestMockDB(t *testing.T, uuid string) *sql.DB {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	mock.MatchExpectationsInOrder(false)
	mock.ExpectPing()
	mock.ExpectPing()
	mock.ExpectQuery(`SELECT @@server_uuid, @@hostname, VERSION\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{"server_uuid", "hostname", "version"}).
			AddRow(uuid, "host-"+uuid, "8.0.0"))
	return db
}

func TestMySQL_MultipleDatabases(t *testing.T) {
	args := Arguments{
		Databases: []DatabaseArguments{
			{Name: "a", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db1"},
			{Name: "b", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db2"},
		},
		DisableCollectors: []string{"query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}

	var gotExports cmp.Exports
	opts := cmp.Options{
		ID:             "test.mysql",
		Logger:         logging.NewSlogNop(),
		GetServiceData: testGetServiceData,
		OnStateChange:  func(e cmp.Exports) { gotExports = e },
	}

	dbA := multiDatabaseTestMockDB(t, "uuid-a")
	dbB := multiDatabaseTestMockDB(t, "uuid-b")

	c, err := new(opts, args, func(_ string, dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, "/db1") {
			return dbA, nil
		}
		return dbB, nil
	})
	require.NoError(t, err)

	h := c.CurrentHealth()
	assert.Equal(t, cmp.HealthTypeHealthy, h.Health)

	exported, ok := gotExports.(Exports)
	require.True(t, ok)
	require.Len(t, exported.Targets, 2)

	instanceA, _ := exported.Targets[0].Get("instance")
	assert.Equal(t, "tcp(127.0.0.1:3306)/db1", instanceA)
	pathA, _ := exported.Targets[0].Get(model.MetricsPathLabel)
	assert.True(t, strings.HasSuffix(pathA, "/db/a/metrics"), "unexpected metrics path %q", pathA)

	instanceB, _ := exported.Targets[1].Get("instance")
	assert.Equal(t, "tcp(127.0.0.1:3306)/db2", instanceB)
	pathB, _ := exported.Targets[1].Get(model.MetricsPathLabel)
	assert.True(t, strings.HasSuffix(pathB, "/db/b/metrics"), "unexpected metrics path %q", pathB)

	// Each database is served on its own metrics path.
	for _, path := range []string{"/db/a/metrics", "/db/b/metrics"} {
		rec := httptest.NewRecorder()
		c.Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "database_observability_connection_info")
	}

	// The single-database /metrics path is not served when database_instance blocks are used.
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMySQL_MultipleDatabases_PartialFailure(t *testing.T) {
	args := Arguments{
		Databases: []DatabaseArguments{
			{Name: "a", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db1"},
			{Name: "b", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db2"},
		},
		DisableCollectors: []string{"query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}

	var gotExports cmp.Exports
	opts := cmp.Options{
		ID:             "test.mysql",
		Logger:         logging.NewSlogNop(),
		GetServiceData: testGetServiceData,
		OnStateChange:  func(e cmp.Exports) { gotExports = e },
	}

	dbA := multiDatabaseTestMockDB(t, "uuid-a")

	dbB, mockB, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { dbB.Close() })
	mockB.ExpectPing().WillReturnError(assert.AnError)

	c, err := new(opts, args, func(_ string, dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, "/db1") {
			return dbA, nil
		}
		return dbB, nil
	})
	require.NoError(t, err)

	h := c.CurrentHealth()
	assert.Equal(t, cmp.HealthTypeUnhealthy, h.Health)
	assert.Contains(t, h.Message, `database "b"`)

	// Only the connected database exports targets.
	exported, ok := gotExports.(Exports)
	require.True(t, ok)
	require.Len(t, exported.Targets, 1)
	instance, _ := exported.Targets[0].Get("instance")
	assert.Equal(t, "tcp(127.0.0.1:3306)/db1", instance)
}

// TestMySQL_Update_FailedRebuildKeepsOldInstances tests that when Update can't
// build the new instances, it returns an error and the previous instances keep
// running untouched.
func TestMySQL_Update_FailedRebuildKeepsOldInstances(t *testing.T) {
	failGetServiceData := false
	opts := cmp.Options{
		ID:     "test.mysql",
		Logger: logging.NewSlogNop(),
		GetServiceData: func(name string) (any, error) {
			if failGetServiceData {
				return nil, assert.AnError
			}
			return testGetServiceData(name)
		},
		OnStateChange: func(e cmp.Exports) {},
	}
	args := Arguments{
		DataSourceName:    "user:pass@tcp(127.0.0.1:3306)/db",
		DisableCollectors: []string{"query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}

	db := multiDatabaseTestMockDB(t, "uuid-1")

	c, err := new(opts, args, func(_ string, _ string) (*sql.DB, error) { return db, nil })
	require.NoError(t, err)

	before := c.loadInstances()
	require.Len(t, before, 1)
	require.NotEmpty(t, before[0].collectors)

	failGetServiceData = true
	err = c.Update(args)
	require.Error(t, err)

	after := c.loadInstances()
	require.Len(t, after, 1)
	assert.Same(t, before[0], after[0])
	assert.NotEmpty(t, after[0].collectors)
	assert.Equal(t, cmp.HealthTypeHealthy, c.CurrentHealth().Health)
}

// fakeCluster is a cluster.Cluster implementation with controllable
// readiness and per-key ownership.
type fakeCluster struct {
	ready bool
	owned map[shard.Key]bool
	err   error
}

func (f *fakeCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []peer.Peer{{Name: "peer", Self: f.owned[key]}}, nil
}

func (f *fakeCluster) Peers() []peer.Peer { return nil }

func (f *fakeCluster) Ready() bool { return f.ready }

var (
	clusterKeyDB1 = shard.StringKey("tcp(127.0.0.1:3306)/db1")
	clusterKeyDB2 = shard.StringKey("tcp(127.0.0.1:3306)/db2")
)

func clusteringTestOptions(t *testing.T, fake *fakeCluster, gotExports *cmp.Exports) cmp.Options {
	t.Helper()

	return cmp.Options{
		ID:     "test.mysql",
		Logger: logging.NewSlogNop(),
		GetServiceData: func(name string) (any, error) {
			if name == cluster.ServiceName {
				return fake, nil
			}
			return testGetServiceData(name)
		},
		OnStateChange: func(e cmp.Exports) { *gotExports = e },
	}
}

func clusteringTestArgs() Arguments {
	return Arguments{
		Databases: []DatabaseArguments{
			{Name: "a", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db1"},
			{Name: "b", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db2"},
		},
		Clustering:        cluster.ComponentBlock{Enabled: true},
		DisableCollectors: []string{"query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}
}

func TestMySQL_Clustering_DistributesDatabases(t *testing.T) {
	fake := &fakeCluster{ready: true, owned: map[shard.Key]bool{clusterKeyDB1: true, clusterKeyDB2: false}}
	var gotExports cmp.Exports

	dbA := multiDatabaseTestMockDB(t, "uuid-a")
	var connected []string
	c, err := new(clusteringTestOptions(t, fake, &gotExports), clusteringTestArgs(), func(_ string, dsn string) (*sql.DB, error) {
		connected = append(connected, dsn)
		return dbA, nil
	})
	require.NoError(t, err)

	// Only the owned database runs and connects.
	instances := c.loadInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, "tcp(127.0.0.1:3306)/db1", instances[0].instanceKey)
	require.Len(t, connected, 1)
	assert.Contains(t, connected[0], "/db1")

	// Only the owned database exports its target.
	exported, ok := gotExports.(Exports)
	require.True(t, ok)
	require.Len(t, exported.Targets, 1)
	instance, _ := exported.Targets[0].Get("instance")
	assert.Equal(t, "tcp(127.0.0.1:3306)/db1", instance)
}

func TestMySQL_Clustering_NotReadyOwnsNothing(t *testing.T) {
	// The hash owns both databases, so only the readiness gate can produce
	// zero instances: this pins the Ready() check specifically.
	fake := &fakeCluster{ready: false, owned: map[shard.Key]bool{clusterKeyDB1: true, clusterKeyDB2: true}}
	var gotExports cmp.Exports

	c, err := new(clusteringTestOptions(t, fake, &gotExports), clusteringTestArgs(), func(_ string, _ string) (*sql.DB, error) {
		t.Error("unexpected database connection while the cluster is not ready")
		return nil, assert.AnError
	})
	require.NoError(t, err)

	assert.Empty(t, c.loadInstances())

	h := c.CurrentHealth()
	assert.Equal(t, cmp.HealthTypeHealthy, h.Health)
	assert.Contains(t, h.Message, "no databases are currently owned")

	exported, ok := gotExports.(Exports)
	require.True(t, ok)
	assert.Empty(t, exported.Targets)
}

func TestMySQL_Clustering_DisabledOwnsEverything(t *testing.T) {
	// The cluster owns nothing locally, but with the clustering block
	// disabled the component must ignore it entirely.
	fake := &fakeCluster{ready: true, owned: map[shard.Key]bool{}}
	var gotExports cmp.Exports

	args := clusteringTestArgs()
	args.Clustering.Enabled = false

	dbA := multiDatabaseTestMockDB(t, "uuid-a")
	dbB := multiDatabaseTestMockDB(t, "uuid-b")
	c, err := new(clusteringTestOptions(t, fake, &gotExports), args, func(_ string, dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, "/db1") {
			return dbA, nil
		}
		return dbB, nil
	})
	require.NoError(t, err)

	assert.Len(t, c.loadInstances(), 2)
}

func TestMySQL_Clustering_ReconcileOnOwnershipChange(t *testing.T) {
	fake := &fakeCluster{ready: true, owned: map[shard.Key]bool{clusterKeyDB1: true, clusterKeyDB2: false}}
	var gotExports cmp.Exports

	dbA := multiDatabaseTestMockDB(t, "uuid-a")
	dbB := multiDatabaseTestMockDB(t, "uuid-b")
	c, err := new(clusteringTestOptions(t, fake, &gotExports), clusteringTestArgs(), func(_ string, dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, "/db1") {
			return dbA, nil
		}
		return dbB, nil
	})
	require.NoError(t, err)

	before := c.loadInstances()
	require.Len(t, before, 1)
	require.Equal(t, "tcp(127.0.0.1:3306)/db1", before[0].instanceKey)

	// Unchanged ownership: reconciling is a no-op that keeps the same instances.
	c.reconcileCluster()
	after := c.loadInstances()
	require.Len(t, after, 1)
	assert.Same(t, before[0], after[0])

	// Ownership moves to db2: the db1 instance stops and db2 starts.
	fake.owned = map[shard.Key]bool{clusterKeyDB1: false, clusterKeyDB2: true}
	c.reconcileCluster()

	after = c.loadInstances()
	require.Len(t, after, 1)
	assert.Equal(t, "tcp(127.0.0.1:3306)/db2", after[0].instanceKey)
	assert.Empty(t, before[0].collectors)

	exported, ok := gotExports.(Exports)
	require.True(t, ok)
	require.Len(t, exported.Targets, 1)
	instance, _ := exported.Targets[0].Get("instance")
	assert.Equal(t, "tcp(127.0.0.1:3306)/db2", instance)
}

func TestMySQL_Clustering_NotifyClusterChange(t *testing.T) {
	fake := &fakeCluster{ready: true, owned: map[shard.Key]bool{clusterKeyDB1: true, clusterKeyDB2: false}}
	var gotExports cmp.Exports

	dbA := multiDatabaseTestMockDB(t, "uuid-a")
	dbB := multiDatabaseTestMockDB(t, "uuid-b")
	c, err := new(clusteringTestOptions(t, fake, &gotExports), clusteringTestArgs(), func(_ string, dsn string) (*sql.DB, error) {
		if strings.Contains(dsn, "/db1") {
			return dbA, nil
		}
		return dbB, nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		_ = c.Run(ctx)
	}()

	// The channel send in NotifyClusterChange orders this write before the
	// reconcile goroutine's read of the fake cluster.
	fake.owned = map[shard.Key]bool{clusterKeyDB1: false, clusterKeyDB2: true}
	c.NotifyClusterChange()

	require.Eventually(t, func() bool {
		instances := c.loadInstances()
		return len(instances) == 1 && instances[0].instanceKey == "tcp(127.0.0.1:3306)/db2"
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case <-runDone:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestMySQL_Clustering_SingleDatabaseStandby(t *testing.T) {
	// The single-DSN form participates in clustering too: a node that
	// doesn't own the database goes standby instead of collecting.
	fake := &fakeCluster{ready: true, owned: map[shard.Key]bool{shard.StringKey("tcp(127.0.0.1:3306)/db"): false}}
	var gotExports cmp.Exports

	args := Arguments{
		DataSourceName:    "user:pass@tcp(127.0.0.1:3306)/db",
		Clustering:        cluster.ComponentBlock{Enabled: true},
		DisableCollectors: []string{"query_details", "schema_details", "query_samples", "setup_consumers", "setup_actors", "explain_plans", "locks"},
		HealthCheckArguments: HealthCheckArguments{
			CollectInterval: 1 * time.Hour,
		},
	}

	c, err := new(clusteringTestOptions(t, fake, &gotExports), args, func(_ string, _ string) (*sql.DB, error) {
		t.Error("unexpected database connection on a non-owning node")
		return nil, assert.AnError
	})
	require.NoError(t, err)

	assert.Empty(t, c.loadInstances())
	h := c.CurrentHealth()
	assert.Equal(t, cmp.HealthTypeHealthy, h.Health)
	assert.Contains(t, h.Message, "no databases are currently owned")
}

func TestMySQL_Clustering_UpdateRecomputesOwnership(t *testing.T) {
	clusterKeyDB3 := shard.StringKey("tcp(127.0.0.1:3306)/db3")
	fake := &fakeCluster{ready: true, owned: map[shard.Key]bool{clusterKeyDB1: true, clusterKeyDB2: false, clusterKeyDB3: true}}
	var gotExports cmp.Exports

	c, err := new(clusteringTestOptions(t, fake, &gotExports), clusteringTestArgs(), func(_ string, dsn string) (*sql.DB, error) {
		switch {
		case strings.Contains(dsn, "/db3"):
			return multiDatabaseTestMockDB(t, "uuid-c"), nil
		default:
			return multiDatabaseTestMockDB(t, "uuid-a"), nil
		}
	})
	require.NoError(t, err)
	require.Len(t, c.loadInstances(), 1)

	// A config reload re-applies the ownership filter to the new database set.
	args := clusteringTestArgs()
	args.Databases = append(args.Databases, DatabaseArguments{Name: "c", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db3"})
	require.NoError(t, c.Update(args))

	instances := c.loadInstances()
	require.Len(t, instances, 2)
	assert.ElementsMatch(t,
		[]string{"tcp(127.0.0.1:3306)/db1", "tcp(127.0.0.1:3306)/db3"},
		[]string{instances[0].instanceKey, instances[1].instanceKey})
}

func TestMySQL_Clustering_LookupErrorFailsOpen(t *testing.T) {
	// Ownership lookup errors fail open to local ownership, so a broken
	// cluster degrades to collecting everything rather than nothing.
	fake := &fakeCluster{ready: true, err: assert.AnError}
	var gotExports cmp.Exports

	c, err := new(clusteringTestOptions(t, fake, &gotExports), clusteringTestArgs(), func(_ string, dsn string) (*sql.DB, error) {
		switch {
		case strings.Contains(dsn, "/db1"):
			return multiDatabaseTestMockDB(t, "uuid-a"), nil
		default:
			return multiDatabaseTestMockDB(t, "uuid-b"), nil
		}
	})
	require.NoError(t, err)

	assert.Len(t, c.loadInstances(), 2)
}

func TestMySQL_Clustering_ReconcileMovesOnlyChangedDatabases(t *testing.T) {
	clusterKeyDB3 := shard.StringKey("tcp(127.0.0.1:3306)/db3")
	fake := &fakeCluster{ready: true, owned: map[shard.Key]bool{clusterKeyDB1: true, clusterKeyDB2: true, clusterKeyDB3: false}}
	var gotExports cmp.Exports

	args := clusteringTestArgs()
	args.Databases = append(args.Databases, DatabaseArguments{Name: "c", DataSourceName: "user:pass@tcp(127.0.0.1:3306)/db3"})

	c, err := new(clusteringTestOptions(t, fake, &gotExports), args, func(_ string, dsn string) (*sql.DB, error) {
		switch {
		case strings.Contains(dsn, "/db1"):
			return multiDatabaseTestMockDB(t, "uuid-a"), nil
		case strings.Contains(dsn, "/db2"):
			return multiDatabaseTestMockDB(t, "uuid-b"), nil
		default:
			return multiDatabaseTestMockDB(t, "uuid-c"), nil
		}
	})
	require.NoError(t, err)

	before := c.loadInstances()
	require.Len(t, before, 2)
	instA, instB := before[0], before[1]
	require.Equal(t, "tcp(127.0.0.1:3306)/db1", instA.instanceKey)
	require.Equal(t, "tcp(127.0.0.1:3306)/db2", instB.instanceKey)

	// db2 moves away and db3 moves in: db1 must keep running untouched.
	fake.owned = map[shard.Key]bool{clusterKeyDB1: true, clusterKeyDB2: false, clusterKeyDB3: true}
	c.reconcileCluster()

	after := c.loadInstances()
	require.Len(t, after, 2)
	assert.Same(t, instA, after[0])
	assert.NotEmpty(t, instA.collectors)
	assert.Equal(t, "tcp(127.0.0.1:3306)/db3", after[1].instanceKey)
	assert.NotEmpty(t, after[1].collectors)
	assert.Empty(t, instB.collectors)

	exported, ok := gotExports.(Exports)
	require.True(t, ok)
	require.Len(t, exported.Targets, 2)
	first, _ := exported.Targets[0].Get("instance")
	second, _ := exported.Targets[1].Get("instance")
	assert.Equal(t, []string{"tcp(127.0.0.1:3306)/db1", "tcp(127.0.0.1:3306)/db3"}, []string{first, second})
}
