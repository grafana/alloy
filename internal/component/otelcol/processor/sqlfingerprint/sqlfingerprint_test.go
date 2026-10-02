package sqlfingerprint

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/service/livedebugging"
	queryfingerprint "github.com/grafana/alloy/sqlfingerprint"
	"github.com/grafana/alloy/syntax"
)

// quietPublisher disables optional debug output for processor tests.
type quietPublisher struct{}

// IsActive reports that no debug client is subscribed.
func (quietPublisher) IsActive(livedebugging.ComponentID) bool { return false }

// PublishIfActive discards debug data without materializing its SQL payload.
func (quietPublisher) PublishIfActive(livedebugging.Data) {}

// buildComponent wires a real exported consumer to a test-controlled downstream.
func buildComponent(t *testing.T, next *fakeconsumer.Consumer) (*Component, otelcol.Consumer, Arguments) {
	t.Helper()
	var args Arguments
	args.SetToDefault()
	args.Output = &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{next}}
	var input otelcol.Consumer
	c, err := New(component.Options{
		ID:             "otelcol.processor.sql_fingerprint.test",
		Registerer:     prometheus.NewRegistry(),
		GetServiceData: func(string) (any, error) { return quietPublisher{}, nil },
		OnStateChange:  func(exports component.Exports) { input = exports.(otelcol.ConsumerExports).Input },
	}, args)
	require.NoError(t, err)
	require.NotNil(t, input)
	return c, input, args
}

// sqlTraces makes an input with timing and unrelated attributes to preserve.
func sqlTraces(systemKey, system, queryKey, query string) ptrace.Traces {
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("database operation")
	span.SetStartTimestamp(10)
	span.SetEndTimestamp(20)
	span.Attributes().PutStr(systemKey, system)
	span.Attributes().PutStr(queryKey, query)
	span.Attributes().PutStr("keep", "unchanged")
	return traces
}

// firstSpan obtains the single span used by pipeline fixtures.
func firstSpan(traces ptrace.Traces) ptrace.Span {
	return traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
}

// fingerprints requires an array even for a single statement.
func fingerprints(t *testing.T, traces ptrace.Traces) []string {
	t.Helper()
	v, ok := firstSpan(traces).Attributes().Get(fingerprintAttribute)
	require.True(t, ok)
	require.Equal(t, pcommon.ValueTypeSlice, v.Type())
	var result []string
	for _, value := range v.Slice().All() {
		require.Equal(t, pcommon.ValueTypeStr, value.Type())
		result = append(result, value.Str())
	}
	return result
}

// TestPipeline checks public-library parity, legacy conventions, deduplicated
// arrays, partial failures, source immutability, and original span preservation.
func TestPipeline(t *testing.T) {
	for _, tt := range []struct {
		systemKey, system, queryKey string
		dialect                     queryfingerprint.Dialect
	}{
		{"db.system.name", "postgresql", "db.query.text", queryfingerprint.PostgreSQL},
		{"db.system", "mysql", "db.statement", queryfingerprint.MySQL},
		{"db.system", "mssql", "db.statement", queryfingerprint.SQLServer},
	} {
		t.Run(tt.system, func(t *testing.T) {
			var received ptrace.Traces
			c, input, _ := buildComponent(t, &fakeconsumer.Consumer{ConsumeTracesFunc: func(_ context.Context, traces ptrace.Traces) error {
				received = traces
				return nil
			}})
			query := "SELECT * FROM a WHERE id=1; SELECT * FROM a WHERE id=2; SELECT * FROM; SELECT * FROM b WHERE id=3"
			traces := sqlTraces(tt.systemKey, tt.system, tt.queryKey, query)
			firstSpan(traces).Attributes().PutStr(fingerprintAttribute, "stale")
			require.NoError(t, input.ConsumeTraces(t.Context(), traces))
			f, err := queryfingerprint.New(queryfingerprint.Options{})
			require.NoError(t, err)
			require.Equal(t, f.Fingerprint(tt.dialect, query).Fingerprints, fingerprints(t, received))
			require.Len(t, fingerprints(t, received), 2)
			stale, _ := firstSpan(traces).Attributes().Get(fingerprintAttribute)
			require.Equal(t, "stale", stale.Str(), "upstream data must not be mutated")
			firstSpan(received).Attributes().PutStr(fingerprintAttribute, "stale")
			require.Equal(t, traces, received, "only the fingerprint attribute should change")
			require.Equal(t, float64(1), testutil.ToFloat64(c.spans.WithLabelValues(string(tt.dialect), "fingerprinted")))
			require.Equal(t, float64(1), testutil.ToFloat64(c.failures.WithLabelValues(string(tt.dialect), "invalid_sql")))
		})
	}
}

// TestPostgresTransactionPipeline checks transaction commands through the trace consumer.
func TestPostgresTransactionPipeline(t *testing.T) {
	for _, query := range []string{"BEGIN", "COMMIT /*action='create',application='TapasFinder',controller='reviews'*/", "SAVEPOINT active_record_1", "BEGIN; SELECT 1; COMMIT"} {
		t.Run(query, func(t *testing.T) {
			var received ptrace.Traces
			c, input, _ := buildComponent(t, &fakeconsumer.Consumer{ConsumeTracesFunc: func(_ context.Context, traces ptrace.Traces) error {
				received = traces
				return nil
			}})
			require.NoError(t, input.ConsumeTraces(t.Context(), sqlTraces("db.system.name", "postgresql", "db.query.text", query)))
			f, err := queryfingerprint.New(queryfingerprint.Options{})
			require.NoError(t, err)
			result := f.Fingerprint(queryfingerprint.PostgreSQL, query)
			require.Empty(t, result.Failures)
			require.NotEmpty(t, result.Fingerprints)
			require.Equal(t, result.Fingerprints, fingerprints(t, received))
			require.Equal(t, float64(0), testutil.ToFloat64(c.failures.WithLabelValues("postgresql", "invalid_sql")))
			require.Equal(t, float64(0), testutil.ToFloat64(c.failures.WithLabelValues("postgresql", "unsupported")))
		})
	}
}

// TestForwardFailures verifies failed fingerprints preserve spans and downstream errors.
func TestForwardFailures(t *testing.T) {
	downstreamError := errors.New("export failed")
	var received ptrace.Traces
	_, input, _ := buildComponent(t, &fakeconsumer.Consumer{ConsumeTracesFunc: func(_ context.Context, traces ptrace.Traces) error {
		received = traces
		return downstreamError
	}})
	for _, tt := range []struct{ system, query string }{{"postgresql", "SELECT 'bad"}, {"postgresql", ""}, {"oracle", "SELECT 1"}, {"", "SELECT 1"}} {
		traces := sqlTraces("db.system.name", tt.system, "db.query.text", tt.query)
		require.ErrorIs(t, input.ConsumeTraces(t.Context(), traces), downstreamError)
		require.Equal(t, traces, received)
	}
	traces := sqlTraces("db.system.name", "mysql", "db.query.text", "SELECT (")
	firstSpan(traces).Attributes().PutStr(fingerprintAttribute, "old")
	require.ErrorIs(t, input.ConsumeTraces(t.Context(), traces), downstreamError)
	_, exists := firstSpan(received).Attributes().Get(fingerprintAttribute)
	require.False(t, exists, "failed recomputation must remove a stale fingerprint")
	require.ErrorIs(t, input.ConsumeMetrics(t.Context(), pmetric.NewMetrics()), pipeline.ErrSignalNotSupported)
}

// TestAttributePrecedence ensures current attributes override legacy values and
// numeric or missing query attributes cannot be treated as a SQL string.
func TestAttributePrecedence(t *testing.T) {
	var received ptrace.Traces
	_, input, _ := buildComponent(t, &fakeconsumer.Consumer{ConsumeTracesFunc: func(_ context.Context, traces ptrace.Traces) error {
		received = traces
		return nil
	}})
	traces := sqlTraces("db.system.name", "mysql", "db.query.text", "SELECT * FROM a")
	attrs := firstSpan(traces).Attributes()
	attrs.PutStr("db.system", "postgresql")
	attrs.PutStr("db.statement", "SELECT * FROM b")
	require.NoError(t, input.ConsumeTraces(t.Context(), traces))
	f, err := queryfingerprint.New(queryfingerprint.Options{})
	require.NoError(t, err)
	require.Equal(t, f.Fingerprint(queryfingerprint.MySQL, "SELECT * FROM a").Fingerprints, fingerprints(t, received))
	attrs.Remove("db.statement")
	attrs.PutInt("db.query.text", 123)
	require.NoError(t, input.ConsumeTraces(t.Context(), traces))
	require.Equal(t, traces, received)
}

// TestReload checks that the exported input survives valid and invalid updates,
// and that the parser modes are forwarded to the shared library.
func TestReload(t *testing.T) {
	var received ptrace.Traces
	c, input, args := buildComponent(t, &fakeconsumer.Consumer{ConsumeTracesFunc: func(_ context.Context, traces ptrace.Traces) error {
		received = traces
		return nil
	}})
	traces := sqlTraces("db.system.name", "mysql", "db.query.text", `SELECT "name" FROM t`)
	require.NoError(t, input.ConsumeTraces(t.Context(), traces))
	before := fingerprints(t, received)
	args.MySQLANSIQuotes = true
	require.NoError(t, c.Update(args))
	require.NoError(t, input.ConsumeTraces(t.Context(), traces))
	after := fingerprints(t, received)
	require.NotEqual(t, before, after)
	f, err := queryfingerprint.New(queryfingerprint.Options{MySQLANSIQuotes: true})
	require.NoError(t, err)
	require.Equal(t, f.Fingerprint(queryfingerprint.MySQL, `SELECT "name" FROM t`).Fingerprints, after)
	args.PostgreSQLVersion = 1
	require.Error(t, c.Update(args))
	require.NoError(t, input.ConsumeTraces(t.Context(), traces))
	require.Equal(t, after, fingerprints(t, received))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, c.Run(ctx))
}

// TestConcurrentReload exercises the consumer swap while traces are flowing.
func TestConcurrentReload(t *testing.T) {
	c, input, args := buildComponent(t, &fakeconsumer.Consumer{})
	var workers sync.WaitGroup
	workers.Go(func() {
		for range 50 {
			args.MySQLANSIQuotes = !args.MySQLANSIQuotes
			if err := c.Update(args); err != nil {
				t.Error(err)
			}
		}
	})
	for range 4 {
		workers.Go(func() {
			for range 50 {
				if err := input.ConsumeTraces(t.Context(), sqlTraces("db.system", "mysql", "db.statement", "SELECT 1")); err != nil {
					t.Error(err)
				}
			}
		})
	}
	workers.Wait()
}

// TestConfiguration verifies Alloy syntax defaults and user-visible validation.
func TestConfiguration(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte("output {}"), &args))
	require.Equal(t, 18, args.PostgreSQLVersion)
	require.Equal(t, 1<<20, args.MaxQueryBytes)
	for _, config := range []string{"", "postgresql_version = 15\noutput {}", "max_query_bytes = -1\noutput {}"} {
		require.Error(t, syntax.Unmarshal([]byte(config), &args))
	}
	args.SetToDefault()
	args.Output = &otelcol.ConsumerArguments{Metrics: []otelcol.Consumer{&fakeconsumer.Consumer{}}}
	require.ErrorContains(t, args.Validate(), "only traces")
}
