package dbo11yreceiver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
)

const (
	dsnA = "postgres://u@db-a:5432/appdb?sslmode=disable"
	dsnB = "postgres://u@db-b:5432/appdb?sslmode=disable"
	keyA = "postgresql://db-a:5432/appdb"
	keyB = "postgresql://db-b:5432/appdb"
)

// fakeDB hands out sqlmock connections that answer the server-info query
// once per Connect.
type fakeDB struct {
	mu      sync.Mutex
	calls   map[string]int
	mocks   map[string][]sqlmock.Sqlmock
	sysIDs  map[string]string
	failing bool
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		calls:  map[string]int{},
		mocks:  map[string][]sqlmock.Sqlmock{},
		sysIDs: map[string]string{dsnA: "1111", dsnB: "2222"},
	}
}

func (f *fakeDB) setFailing(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = v
}

func (f *fakeDB) open(_, dsn string) (*sql.DB, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[dsn]++
	if f.failing {
		return nil, errors.New("connection refused")
	}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		return nil, err
	}
	mock.ExpectQuery("pg_control_system").WillReturnRows(
		sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
			AddRow(f.sysIDs[dsn], "10.0.0.1", "5432", "15.0"))
	f.mocks[dsn] = append(f.mocks[dsn], mock)
	return db, nil
}

func (f *fakeDB) openCalls(dsn string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[dsn]
}

// wantServerID is what dbinstance computes for the fake server info.
func (f *fakeDB) wantServerID(dsn string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(f.sysIDs[dsn]+":10.0.0.1:5432")))
}

func testConfig(dsns ...string) *Config {
	cfg := createDefaultConfig()
	for _, dsn := range dsns {
		cfg.Databases = append(cfg.Databases, DatabaseConfig{DataSourceName: configopaque.String(dsn)})
	}
	cfg.ExcludeCurrentUser = false
	cfg.DisableCollectors = []string{
		collector.QueryDetailsCollector, collector.QuerySamplesCollector,
		collector.SchemaDetailsCollector, collector.ExplainPlanCollector,
	}
	cfg.Exporter.Disabled = true
	cfg.ScrapeInterval = 100 * time.Millisecond
	cfg.ReconnectInterval = 100 * time.Millisecond
	return cfg
}

func settings() receiver.Settings {
	return receivertest.NewNopSettings(typeStr)
}

type started struct {
	t    *testing.T
	recv []component.Component
}

func (s started) shutdown() {
	for _, r := range s.recv {
		require.NoError(s.t, r.Shutdown(context.Background()))
	}
}

func startBoth(t *testing.T, f receiver.Factory, cfg *Config, host component.Host, ms consumer.Metrics, ls consumer.Logs) started {
	t.Helper()
	mr, err := f.CreateMetrics(context.Background(), settings(), cfg, ms)
	require.NoError(t, err)
	lr, err := f.CreateLogs(context.Background(), settings(), cfg, ls)
	require.NoError(t, err)
	require.NoError(t, mr.Start(context.Background(), host))
	require.NoError(t, lr.Start(context.Background(), host))
	return started{t: t, recv: []component.Component{mr, lr}}
}

// forEachDatapoint calls fn with the resource and attributes of every datapoint.
func forEachDatapoint(mds []pmetric.Metrics, fn func(name string, res, attrs pcommon.Map)) {
	for _, md := range mds {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				ms := rm.ScopeMetrics().At(j).Metrics()
				for k := 0; k < ms.Len(); k++ {
					m := ms.At(k)
					var attrs []pcommon.Map
					switch m.Type() {
					case pmetric.MetricTypeGauge:
						for d := 0; d < m.Gauge().DataPoints().Len(); d++ {
							attrs = append(attrs, m.Gauge().DataPoints().At(d).Attributes())
						}
					case pmetric.MetricTypeSum:
						for d := 0; d < m.Sum().DataPoints().Len(); d++ {
							attrs = append(attrs, m.Sum().DataPoints().At(d).Attributes())
						}
					case pmetric.MetricTypeHistogram:
						for d := 0; d < m.Histogram().DataPoints().Len(); d++ {
							attrs = append(attrs, m.Histogram().DataPoints().At(d).Attributes())
						}
					case pmetric.MetricTypeSummary:
						for d := 0; d < m.Summary().DataPoints().Len(); d++ {
							attrs = append(attrs, m.Summary().DataPoints().At(d).Attributes())
						}
					}
					for _, a := range attrs {
						fn(m.Name(), rm.Resource().Attributes(), a)
					}
				}
			}
		}
	}
}

func forEachLogRecord(lds []plog.Logs, fn func(res, attrs pcommon.Map)) {
	for _, ld := range lds {
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			rl := ld.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				lrs := rl.ScopeLogs().At(j).LogRecords()
				for k := 0; k < lrs.Len(); k++ {
					fn(rl.Resource().Attributes(), lrs.At(k).Attributes())
				}
			}
		}
	}
}

func str(m pcommon.Map, key string) string {
	v, ok := m.Get(key)
	if !ok {
		return ""
	}
	return v.Str()
}

func waitForData(t *testing.T, ms *consumertest.MetricsSink, ls *consumertest.LogsSink) {
	t.Helper()
	require.Eventually(t, func() bool {
		return ms.DataPointCount() > 0 && (ls == nil || ls.LogRecordCount() > 0)
	}, 5*time.Second, 20*time.Millisecond)
}

func TestSharedInstanceAcrossSignals(t *testing.T) {
	fdb := newFakeDB()
	f := newFactory(fdb.open)
	cfg := testConfig(dsnA)
	ms, ls := new(consumertest.MetricsSink), new(consumertest.LogsSink)

	s := startBoth(t, f, cfg, componenttest.NewNopHost(), ms, ls)
	waitForData(t, ms, ls)
	s.shutdown()

	require.Equal(t, 1, fdb.openCalls(dsnA), "one connection for both signals")
	require.Len(t, fdb.mocks[dsnA], 1)
	require.NoError(t, fdb.mocks[dsnA][0].ExpectationsWereMet(), "the one server-info query ran")

	ids := map[string]struct{}{}
	forEachDatapoint(ms.AllMetrics(), func(_ string, res, _ pcommon.Map) { ids[str(res, "server_id")] = struct{}{} })
	forEachLogRecord(ls.AllLogs(), func(res, _ pcommon.Map) { ids[str(res, "server_id")] = struct{}{} })
	require.Equal(t, map[string]struct{}{fdb.wantServerID(dsnA): {}}, ids)
}

func TestTwoDatabasesTwoServerIDs(t *testing.T) {
	fdb := newFakeDB()
	f := newFactory(fdb.open)
	ms, ls := new(consumertest.MetricsSink), new(consumertest.LogsSink)

	s := startBoth(t, f, testConfig(dsnA, dsnB), componenttest.NewNopHost(), ms, ls)
	require.Eventually(t, func() bool {
		seen := map[string]bool{}
		forEachLogRecord(ls.AllLogs(), func(res, _ pcommon.Map) { seen[str(res, attrServiceInstanceID)] = true })
		forEachDatapoint(ms.AllMetrics(), func(_ string, res, _ pcommon.Map) { seen["m"+str(res, attrServiceInstanceID)] = true })
		return seen[keyA] && seen[keyB] && seen["m"+keyA] && seen["m"+keyB]
	}, 5*time.Second, 20*time.Millisecond)
	s.shutdown()

	want := map[string]map[string]struct{}{
		keyA: {fdb.wantServerID(dsnA): {}},
		keyB: {fdb.wantServerID(dsnB): {}},
	}
	got := map[string]map[string]struct{}{}
	add := func(res, attrs pcommon.Map) {
		key := str(res, attrServiceInstanceID)
		if got[key] == nil {
			got[key] = map[string]struct{}{}
		}
		got[key][str(res, "server_id")] = struct{}{}
		require.Equal(t, key, str(attrs, "instance"), "log instance label matches the resource")
	}
	forEachLogRecord(ls.AllLogs(), add)
	gotLogs := got
	require.Equal(t, want, gotLogs)

	got = map[string]map[string]struct{}{}
	forEachDatapoint(ms.AllMetrics(), func(_ string, res, _ pcommon.Map) {
		key := str(res, attrServiceInstanceID)
		if got[key] == nil {
			got[key] = map[string]struct{}{}
		}
		got[key][str(res, "server_id")] = struct{}{}
	})
	require.Equal(t, want, got)
	require.NotEqual(t, fdb.wantServerID(dsnA), fdb.wantServerID(dsnB))
}

// statusHost records the status events the receiver reports.
type statusHost struct {
	component.Host
	mu     sync.Mutex
	events []*componentstatus.Event
}

func (h *statusHost) Report(e *componentstatus.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, e)
}

func (h *statusHost) last() *componentstatus.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.events) == 0 {
		return nil
	}
	return h.events[len(h.events)-1]
}

func TestStartsWhenDatabaseUnreachable(t *testing.T) {
	ignore := goleak.IgnoreCurrent()

	fdb := newFakeDB()
	fdb.setFailing(true)
	f := newFactory(fdb.open)
	cfg := testConfig(dsnA)
	cfg.ScrapeInterval = 300 * time.Millisecond
	cfg.ReconnectInterval = 300 * time.Millisecond
	host := &statusHost{Host: componenttest.NewNopHost()}
	ms := new(consumertest.MetricsSink)

	mr, err := f.CreateMetrics(context.Background(), settings(), cfg, ms)
	require.NoError(t, err)
	require.NoError(t, mr.Start(context.Background(), host))

	require.Eventually(t, func() bool {
		e := host.last()
		return e != nil && e.Status() == componentstatus.StatusRecoverableError &&
			strings.Contains(e.Err().Error(), "connection refused")
	}, 5*time.Second, 20*time.Millisecond)
	require.GreaterOrEqual(t, fdb.openCalls(dsnA), 1)
	require.Zero(t, ms.DataPointCount())

	fdb.setFailing(false)
	require.Eventually(t, func() bool { return ms.DataPointCount() > 0 },
		3*cfg.ScrapeInterval, 10*time.Millisecond, "metrics within 3 intervals of the database coming back")
	require.Eventually(t, func() bool {
		e := host.last()
		return e != nil && e.Status() == componentstatus.StatusOK
	}, 5*time.Second, 20*time.Millisecond)

	require.NoError(t, mr.Shutdown(context.Background()))
	goleak.VerifyNone(t, ignore)
}

func TestShutdownWithBlockedConsumer(t *testing.T) {
	ignore := goleak.IgnoreCurrent()

	fdb := newFakeDB()
	f := newFactory(fdb.open)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	blocked, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release // ignores ctx: never returns until the test says so
		return nil
	})
	require.NoError(t, err)

	lr, err := f.CreateLogs(context.Background(), settings(), testConfig(dsnA), blocked)
	require.NoError(t, err)
	require.NoError(t, lr.Start(context.Background(), componenttest.NewNopHost()))

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("logs consumer never called")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	begin := time.Now()
	_ = lr.Shutdown(ctx) // reports the stuck consumer, which is fine
	require.Less(t, time.Since(begin), 3*time.Second)

	// A consumer that never returns keeps the pump goroutine alive (F-22), so
	// this only proves nothing else leaks once it does return.
	close(release)
	goleak.VerifyNone(t, ignore)
}

func TestResourceAttributes(t *testing.T) {
	fdb := newFakeDB()
	f := newFactory(fdb.open)
	ms, ls := new(consumertest.MetricsSink), new(consumertest.LogsSink)

	s := startBoth(t, f, testConfig(dsnA), componenttest.NewNopHost(), ms, ls)
	waitForData(t, ms, ls)
	s.shutdown()

	wantID := fdb.wantServerID(dsnA)
	checkRes := func(res pcommon.Map) {
		require.Equal(t, "integrations/db-o11y", str(res, attrServiceName))
		require.Equal(t, keyA, str(res, attrServiceInstanceID))
		require.Equal(t, wantID, str(res, "server_id"), "side a: server_id is a resource attribute")
	}
	forEachDatapoint(ms.AllMetrics(), func(name string, res, attrs pcommon.Map) {
		checkRes(res)
		_, hasID := attrs.Get("server_id")
		require.False(t, hasID, name)
		_, hasJob := attrs.Get("job")
		require.False(t, hasJob, name)
	})
	forEachLogRecord(ls.AllLogs(), func(res, attrs pcommon.Map) {
		checkRes(res)
		_, hasID := attrs.Get("server_id")
		require.False(t, hasID)
		require.Equal(t, "integrations/db-o11y", str(attrs, "job"))
		require.Equal(t, keyA, str(attrs, "instance"))
	})
}

func metricNames(mds []pmetric.Metrics) map[string]bool {
	names := map[string]bool{}
	forEachDatapoint(mds, func(name string, _, _ pcommon.Map) { names[name] = true })
	return names
}

func TestExporterDisabledNoPgMetrics(t *testing.T) {
	// An unreachable exporter DSN still yields the exporter's own pg_* series,
	// which makes the disabled case meaningful.
	const dsn = "postgres://u@127.0.0.1:1/postgres?sslmode=disable"
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("disabled=%v", disabled), func(t *testing.T) {
			fdb := newFakeDB()
			f := newFactory(fdb.open)
			cfg := testConfig(dsn)
			cfg.Exporter.Disabled = disabled
			ms := new(consumertest.MetricsSink)
			mr, err := f.CreateMetrics(context.Background(), settings(), cfg, ms)
			require.NoError(t, err)
			require.NoError(t, mr.Start(context.Background(), componenttest.NewNopHost()))
			require.Eventually(t, func() bool { return ms.DataPointCount() > 0 }, 15*time.Second, 20*time.Millisecond)
			require.NoError(t, mr.Shutdown(context.Background()))

			wantID := fdb.wantServerID(dsn)
			forEachDatapoint(ms.AllMetrics(), func(name string, res, _ pcommon.Map) {
				require.Equal(t, wantID, str(res, "server_id"), name)
			})

			names := metricNames(ms.AllMetrics())
			require.True(t, names["database_observability_connection_info"], "db-o11y metrics flow")
			hasPg := false
			for n := range names {
				if strings.HasPrefix(n, "pg_") {
					hasPg = true
				}
			}
			require.Equal(t, !disabled, hasPg, "%v", names)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := createDefaultConfig()
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
	require.ErrorContains(t, cfg.Validate(), "at least one")

	cfg = testConfig(dsnA, dsnA)
	require.ErrorContains(t, cfg.Validate(), "same server")

	cfg = testConfig(dsnA)
	cfg.Databases[0].CloudProvider = &CloudProviderConfig{}
	cfg.Databases[0].CloudProvider.AWS = &struct {
		ARN string `mapstructure:"arn"`
	}{ARN: "arn"}
	cfg.Databases[0].CloudProvider.GCP = &struct {
		ConnectionName string `mapstructure:"connection_name"`
	}{ConnectionName: "p:r:i"}
	require.ErrorContains(t, cfg.Validate(), "at most one of aws, azure, or gcp")

	cfg = testConfig(dsnA)
	cfg.Exporter.Disabled = false
	cfg.Exporter.DisableDefaultMetrics = true
	require.ErrorContains(t, cfg.Validate(), "custom_queries_config_path must be set")
	cfg.Exporter.CustomQueriesConfigPath = "/q.yaml"
	require.NoError(t, cfg.Validate())
	cfg.Exporter.EnabledCollectors = []string{"stat_statements"}
	require.ErrorContains(t, cfg.Validate(), "enabled_collectors cannot be set")

	cfg = testConfig(dsnA)
	require.NoError(t, cfg.Validate())
	require.Equal(t, 15*time.Second, createDefaultConfig().ScrapeInterval)
}
