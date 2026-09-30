package sqlfingerprint

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
)

// A prepared-statement execution carries the cached, obfuscated SQL. Every SQL
// span must get the same fingerprint, regardless of its scope or position.
func TestRubyObfuscatedParametersAcrossSpans(t *testing.T) {
	var received ptrace.Traces
	c, input, _ := buildComponent(t, &fakeconsumer.Consumer{ConsumeTracesFunc: func(_ context.Context, traces ptrace.Traces) error {
		received = traces
		return nil
	}})
	traces := ptrace.NewTraces()
	resource := traces.ResourceSpans().AppendEmpty()
	resource.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("HTTP request")
	queries := []string{
		`SELECT AVG("reviews"."rating") FROM "reviews" WHERE "reviews"."restaurant_id" = $1`,
		`SELECT AVG("reviews"."rating") FROM "reviews" WHERE "reviews"."restaurant_id" = $?`,
		`SELECT AVG("reviews"."rating") FROM "reviews" WHERE "reviews"."restaurant_id" = $? ?`,
	}
	for range 2 {
		scope := resource.ScopeSpans().AppendEmpty()
		for _, query := range queries {
			span := scope.Spans().AppendEmpty()
			span.SetTraceID(pcommon.TraceID{1})
			span.Attributes().PutStr("db.system", "postgresql")
			span.Attributes().PutStr("db.statement", query)
			span.Attributes().PutStr("db.postgresql.prepared_statement_name", "a1")
		}
	}
	require.NoError(t, input.ConsumeTraces(t.Context(), traces))
	var expected string
	scopes := received.ResourceSpans().At(0).ScopeSpans()
	_, exists := scopes.At(0).Spans().At(0).Attributes().Get(fingerprintAttribute)
	require.False(t, exists)
	for i := 1; i < scopes.Len(); i++ {
		for j, span := range scopes.At(i).Spans().All() {
			attrs := span.Attributes()
			fp, ok := attrs.Get(fingerprintAttribute)
			require.True(t, ok)
			require.Equal(t, pcommon.ValueTypeSlice, fp.Type())
			require.Equal(t, 1, fp.Slice().Len())
			if expected == "" {
				expected = fp.Slice().At(0).Str()
			}
			require.Equal(t, expected, fp.Slice().At(0).Str())
			sql, _ := attrs.Get("db.statement")
			require.Equal(t, queries[j], sql.Str())
		}
	}
	require.Equal(t, float64(6), testutil.ToFloat64(c.spans.WithLabelValues("postgresql", "fingerprinted")))
	require.Equal(t, float64(0), testutil.ToFloat64(c.spans.WithLabelValues("postgresql", "skipped")))
}
