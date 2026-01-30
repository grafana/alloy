package mysql

import (
	"database/sql"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kitlog "github.com/go-kit/log"
	cmp "github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
)

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
		ID:     "test.mysql",
		Logger: kitlog.NewNopLogger(),
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
		ID:     "test.mysql",
		Logger: kitlog.NewNopLogger(),
		GetServiceData: func(name string) (any, error) {
			return http_service.Data{MemoryListenAddr: "127.0.0.1:0", BaseHTTPPath: "/component"}, nil
		},
		OnStateChange: func(e cmp.Exports) { gotExports = e },
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
