package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
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
	exporter_postgres "github.com/grafana/alloy/internal/component/prometheus/exporter/postgres"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/loki/pkg/push"
)

func newTestComponent(t *testing.T, openSQL func(string, string) (*sql.DB, error)) *Component {
	t.Helper()
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
	c := &Component{
		opts:         opts,
		args:         args,
		fanout:       loki.NewFanout(args.ForwardTo),
		handler:      loki.NewLogsReceiver(),
		registry:     prometheus.NewRegistry(),
		healthErr:    atomic.NewString(""),
		openSQL:      openSQL,
		logsReceiver: loki.NewLogsReceiver(),
	}
	c.instanceKey = "test-instance"
	c.baseTarget = discovery.NewTargetFromMap(map[string]string{
		"instance": c.instanceKey,
		"job":      "database_observability",
	})
	return c
}

func Test_defaultExclusions(t *testing.T) {
	exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
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
	}, args.ExcludeDatabases)

	assert.Equal(t, []string{
		"azuresu",
		"cloudsqladmin",
		"db-o11y",
		"rdsadmin",
	}, args.ExcludeUsers)

	assert.True(t, args.ExcludeCurrentUser, "exclude_current_user should default to true")
	assert.Nil(t, args.QuerySampleArguments.ExcludeCurrentUser, "query_samples.exclude_current_user should default to unset")
}

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

func TestQuerySamples_ExcludeCurrentUser_ConfigParsing(t *testing.T) {
	t.Run("unset by default", func(t *testing.T) {
		cfg := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		`
		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
		assert.Nil(t, args.QuerySampleArguments.ExcludeCurrentUser, "should default to nil (unset)")
	})

	t.Run("explicitly true", func(t *testing.T) {
		cfg := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		query_samples {
			exclude_current_user = true
		}
		`
		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
		require.NotNil(t, args.QuerySampleArguments.ExcludeCurrentUser)
		assert.True(t, *args.QuerySampleArguments.ExcludeCurrentUser)
	})

	t.Run("explicitly false", func(t *testing.T) {
		cfg := `
		data_source_name = "postgres://db"
		forward_to = []
		targets = []
		query_samples {
			exclude_current_user = false
		}
		`
		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
		require.NotNil(t, args.QuerySampleArguments.ExcludeCurrentUser)
		assert.False(t, *args.QuerySampleArguments.ExcludeCurrentUser)
	})
}

func TestPostgres_ExcludeCurrentUser_Runtime(t *testing.T) {
	t.Run("when true, queries current_user and merges it into effective exclude list", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPing()
		mock.ExpectQuery(selectServerInfo).
			WillReturnRows(sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
				AddRow("1234567890", "127.0.0.1", "5432", "14.0"))
		mock.ExpectQuery(`SELECT current_user`).
			WillReturnRows(sqlmock.NewRows([]string{"current_user"}).AddRow("alloy_monitor"))

		c := newTestComponent(t, func(_, _ string) (*sql.DB, error) { return db, nil })
		c.args.ExcludeCurrentUser = true
		c.args.ExcludeUsers = []string{"rdsadmin"}

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("when false, current_user is not queried", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPing()
		mock.ExpectQuery(selectServerInfo).
			WillReturnRows(sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
				AddRow("1234567890", "127.0.0.1", "5432", "14.0"))

		c := newTestComponent(t, func(_, _ string) (*sql.DB, error) { return db, nil })
		c.args.ExcludeCurrentUser = false

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns wrapped error when current_user query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPing()
		mock.ExpectQuery(selectServerInfo).
			WillReturnRows(sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
				AddRow("1234567890", "127.0.0.1", "5432", "14.0"))
		mock.ExpectQuery(`SELECT current_user`).WillReturnError(assert.AnError)

		c := newTestComponent(t, func(_, _ string) (*sql.DB, error) { return db, nil })
		c.args.ExcludeCurrentUser = true

		err = c.connectAndStartCollectors(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query current_user")
	})
}

func TestQuerySamples_ExcludeCurrentUser_LocalPrecedence(t *testing.T) {
	ptrBool := func(b bool) *bool { return &b }

	serverInfoRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
			AddRow("1234567890", "127.0.0.1", "5432", "14.0")
	}
	currentUserRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"current_user"}).AddRow("alloy_monitor")
	}
	emptyActivityRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"now", "datname", "pid", "leader_pid",
			"usename", "application_name", "client_addr", "client_port",
			"backend_type", "backend_start", "backend_xid", "backend_xmin",
			"xact_start", "state", "state_change", "wait_event_type",
			"wait_event", "blocked_by_pids", "query_start", "query_id",
		})
	}

	setup := func(t *testing.T) (*Component, sqlmock.Sqlmock, *sql.DB) {
		t.Helper()
		db, mock, err := sqlmock.New(
			sqlmock.MonitorPingsOption(true),
			sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
		)
		require.NoError(t, err)
		mock.MatchExpectationsInOrder(false)

		c := newTestComponent(t, func(_, _ string) (*sql.DB, error) { return db, nil })
		c.args.EnableCollectors = []string{"query_samples"}
		c.args.DisableCollectors = []string{"query_details", "schema_details", "explain_plans"}
		// Avoid scraping more than once during the test.
		c.args.QuerySampleArguments.CollectInterval = time.Hour
		return c, mock, db
	}

	expectPrelude := func(mock sqlmock.Sqlmock, mockSelectCurrentUser bool) {
		mock.ExpectPing()
		mock.ExpectQuery(regexp.QuoteMeta(selectServerInfo)).WillReturnRows(serverInfoRows())
		if mockSelectCurrentUser {
			mock.ExpectQuery(regexp.QuoteMeta("SELECT current_user")).WillReturnRows(currentUserRows())
		}
	}

	// regexes for the query_samples collector to verify users clause
	const (
		regexNoCurrentUser   = `AND d\.datname NOT IN \('azure_maintenance'\)\s*AND s\.usename NOT IN \(%s\)\s*\z`
		regexWithCurrentUser = `AND d\.datname NOT IN \('azure_maintenance'\)\s*AND s\.usesysid != \(select oid from pg_roles where rolname = current_user\)\s*AND s\.usename NOT IN \(%s\)\s*\z`
	)

	t.Run("local override unset inherits top-level cascade", func(t *testing.T) {
		c, mock, db := setup(t)
		defer db.Close()
		c.args.ExcludeCurrentUser = true
		c.args.ExcludeUsers = []string{"rdsadmin"}
		c.args.QuerySampleArguments.ExcludeCurrentUser = nil

		expectPrelude(mock, true)
		// effectiveExcludeUsers = ['rdsadmin', 'alloy_monitor'], no current_user clause.
		mock.ExpectQuery(`(?s)` + fmt.Sprintf(regexNoCurrentUser, `'rdsadmin', 'alloy_monitor'`)).
			WillReturnRows(emptyActivityRows())

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("local override true forces deprecated SQL-side path", func(t *testing.T) {
		c, mock, db := setup(t)
		defer db.Close()
		c.args.ExcludeCurrentUser = true
		c.args.ExcludeUsers = []string{"rdsadmin"}
		c.args.QuerySampleArguments.ExcludeCurrentUser = ptrBool(true)

		expectPrelude(mock, true)
		// SQL contains the current_user clause, and uses raw c.args.ExcludeUsers
		mock.ExpectQuery(`(?s)` + fmt.Sprintf(regexWithCurrentUser, `'rdsadmin'`)).
			WillReturnRows(emptyActivityRows())

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("local override false opts out and bypasses cascade", func(t *testing.T) {
		c, mock, db := setup(t)
		defer db.Close()
		c.args.ExcludeCurrentUser = true
		c.args.ExcludeUsers = []string{"rdsadmin"}
		c.args.QuerySampleArguments.ExcludeCurrentUser = ptrBool(false)

		expectPrelude(mock, true)
		// No current_user clause, uses raw c.args.ExcludeUsers (no alloy_monitor appended).
		mock.ExpectQuery(`(?s)` + fmt.Sprintf(regexNoCurrentUser, `'rdsadmin'`)).
			WillReturnRows(emptyActivityRows())

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("top-level false with local override true engages SQL clause without lookup", func(t *testing.T) {
		c, mock, db := setup(t)
		defer db.Close()
		c.args.ExcludeCurrentUser = false
		c.args.ExcludeUsers = []string{"rdsadmin"}
		c.args.QuerySampleArguments.ExcludeCurrentUser = ptrBool(true)

		// Top-level is false, so SELECT current_user is NOT issued.
		expectPrelude(mock, false)
		// SQL still contains the current_user clause via the local override.
		mock.ExpectQuery(`(?s)` + fmt.Sprintf(regexWithCurrentUser, `'rdsadmin'`)).
			WillReturnRows(emptyActivityRows())

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("top-level false with local override unset is a no-op baseline", func(t *testing.T) {
		c, mock, db := setup(t)
		defer db.Close()
		c.args.ExcludeCurrentUser = false
		c.args.ExcludeUsers = []string{"rdsadmin"}
		c.args.QuerySampleArguments.ExcludeCurrentUser = nil

		// Top-level is false, so SELECT current_user is NOT issued.
		expectPrelude(mock, false)
		// Neither knob is on: no current_user clause, raw c.args.ExcludeUsers.
		mock.ExpectQuery(`(?s)` + fmt.Sprintf(regexNoCurrentUser, `'rdsadmin'`)).
			WillReturnRows(emptyActivityRows())

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("top-level false with local override false renders identically to baseline", func(t *testing.T) {
		c, mock, db := setup(t)
		defer db.Close()
		c.args.ExcludeCurrentUser = false
		c.args.ExcludeUsers = []string{"rdsadmin"}
		c.args.QuerySampleArguments.ExcludeCurrentUser = ptrBool(false)

		// Top-level is false, so SELECT current_user is NOT issued.
		expectPrelude(mock, false)
		// Same observable SQL as the local=nil baseline above; the deprecated
		// branch is taken (warning logged) but the rendered query is unchanged.
		mock.ExpectQuery(`(?s)` + fmt.Sprintf(regexNoCurrentUser, `'rdsadmin'`)).
			WillReturnRows(emptyActivityRows())

		require.NoError(t, c.connectAndStartCollectors(context.Background()))
		require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 50*time.Millisecond)
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
		assert.Equal(t, defaultArguments().QuerySampleArguments.CollectInterval, args.QuerySampleArguments.CollectInterval, "collect_interval for query_samples should default to 15 seconds")
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

		assert.Equal(t, defaultArguments().SchemaDetailsArguments.CacheEnabled, args.SchemaDetailsArguments.CacheEnabled)
		assert.Equal(t, defaultArguments().SchemaDetailsArguments.CacheSize, args.SchemaDetailsArguments.CacheSize)
		assert.Equal(t, defaultArguments().SchemaDetailsArguments.CacheTTL, args.SchemaDetailsArguments.CacheTTL)
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

	t.Run("parse gcp cloud provider block", func(t *testing.T) {
		exampleDBO11yAlloyConfig := `
		data_source_name = "postgres://db"
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
		data_source_name = "postgres://db"
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
		data_source_name = "postgres://db"
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

func Test_LogsReceiver_ExportedImmediately(t *testing.T) {
	var exports Exports
	opts := cmp.Options{
		ID:         "test",
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
		ForwardTo:      []loki.LogsReceiver{},
		Targets:        []discovery.Target{},
	}

	c, err := New(opts, args)
	require.NoError(t, err)

	require.NotNil(t, exports.LogsReceiver, "LogsReceiver should be exported immediately")
	require.NotNil(t, c.logsReceiver, "component should have logsReceiver initialized")
	assert.Equal(t, c.logsReceiver, exports.LogsReceiver)
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

type fakeClosableCollector struct {
	prometheus.Collector
	closeCalls int
}

func newFakeClosableCollector(name string) *fakeClosableCollector {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: name,
	})
	return &fakeClosableCollector{Collector: gauge}
}

func (c *fakeClosableCollector) CloseServers() {
	c.closeCalls++
}

func TestComponent_cleanupExporterCollectors(t *testing.T) {
	t.Run("closes closable exporters and unregisters them", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		collector := newFakeClosableCollector("test_cleanup_exporter_collectors_closable")
		require.NoError(t, registry.Register(collector))

		c := &Component{
			registry:           registry,
			exporterCollectors: []prometheus.Collector{collector},
		}

		c.cleanupExporterCollectors()

		assert.Equal(t, 1, collector.closeCalls)
		assert.Nil(t, c.exporterCollectors)
		assert.False(t, registry.Unregister(collector))
	})

	t.Run("unregisters non-closable collectors without panicking", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		collector := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "test_cleanup_exporter_collectors_plain",
			Help: "test",
		})
		collector.Set(1)
		require.NoError(t, registry.Register(collector))

		c := &Component{
			registry:           registry,
			exporterCollectors: []prometheus.Collector{collector},
		}

		c.cleanupExporterCollectors()

		assert.Nil(t, c.exporterCollectors)
		assert.False(t, registry.Unregister(collector))
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
		// First mock: will fail
		db1, mock1, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db1.Close()

		mock1.ExpectPing().WillReturnError(assert.AnError)

		c := newTestComponent(t, func(_, _ string) (*sql.DB, error) { return db1, nil })

		// First attempt: connection fails
		err = c.tryReconnect(context.Background())
		assert.Error(t, err)
		assert.NotEmpty(t, c.healthErr.Load())

		// Second mock: will succeed
		db2, mock2, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db2.Close()
		mock2.ExpectPing()
		mock2.ExpectQuery(selectServerInfo).
			WillReturnRows(sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
				AddRow("1234567890", "127.0.0.1", "5432", "14.0"))
		c.openSQL = func(_ string, _ string) (*sql.DB, error) { return db2, nil }

		// Second attempt: connection succeeds and clears error
		err = c.tryReconnect(context.Background())
		assert.NoError(t, err)
		assert.Empty(t, c.healthErr.Load())
	})

	t.Run("Run exits on context cancellation", func(t *testing.T) {
		c := newTestComponent(t, func(_, _ string) (*sql.DB, error) { return nil, assert.AnError })
		oldCollector := newFakeClosableCollector("test_run_cleanup_old_exporter")
		require.NoError(t, c.registry.Register(oldCollector))
		c.exporterCollectors = []prometheus.Collector{oldCollector}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := c.Run(ctx)
		assert.NoError(t, err)

		assert.Equal(t, 1, oldCollector.closeCalls)
		assert.Nil(t, c.exporterCollectors)
		assert.False(t, c.registry.Unregister(oldCollector))
	})
}

func Test_PrometheusExporterBlock(t *testing.T) {
	t.Run("absent when not specified", func(t *testing.T) {
		cfg := `
			data_source_name = "postgresql://user:pass@localhost:5432/db"
			forward_to = []
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.NoError(t, err)
		assert.Nil(t, args.PrometheusExporter)
	})

	t.Run("present with defaults when empty block", func(t *testing.T) {
		cfg := `
			data_source_name = "postgresql://user:pass@localhost:5432/db"
			forward_to = []
			prometheus_exporter {}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.NoError(t, err)
		require.NotNil(t, args.PrometheusExporter)
		exporterArgs := exporter_postgres.Arguments(*args.PrometheusExporter)
		assert.False(t, exporterArgs.DisableDefaultMetrics)
		assert.False(t, exporterArgs.DisableSettingsMetrics)
	})

	t.Run("present with explicit config", func(t *testing.T) {
		cfg := `
			data_source_name = "postgresql://user:pass@localhost:5432/db"
			forward_to = []
			prometheus_exporter {
				disable_settings_metrics = true
			}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.NoError(t, err)
		require.NotNil(t, args.PrometheusExporter)
		exporterArgs := exporter_postgres.Arguments(*args.PrometheusExporter)
		assert.True(t, exporterArgs.DisableSettingsMetrics)
	})

	t.Run("error when both prometheus_exporter and targets are set", func(t *testing.T) {
		cfg := `
			data_source_name = "postgresql://user:pass@localhost:5432/db"
			forward_to = []
			targets = [{"__address__" = "localhost:9187"}]
			prometheus_exporter {}
		`
		var args Arguments
		err := syntax.Unmarshal([]byte(cfg), &args)
		require.ErrorContains(t, err, "prometheus_exporter and targets are mutually exclusive")
	})
}
