package collector

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestQueryHashMetricsCollector_ExposesRegistryAsJoinMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	qhr := NewQueryHashRegistry(100, time.Hour)
	col := NewQueryHashMetricsCollector(qhr, "server-A")
	require.NoError(t, reg.Register(col))

	qhr.Set("12345", "fpAAAA", "books_store")
	qhr.Set("67890", "fpBBBB", "library")

	expected := `
# HELP database_observability_query_hash_info Mapping of PostgreSQL queryid to semantic query fingerprint
# TYPE database_observability_query_hash_info gauge
database_observability_query_hash_info{datname="books_store",query_fingerprint="fpAAAA",queryid="12345",server_id="server-A"} 1
database_observability_query_hash_info{datname="library",query_fingerprint="fpBBBB",queryid="67890",server_id="server-A"} 1
`

	require.NoError(t, testutil.GatherAndCompare(
		reg,
		strings.NewReader(expected),
		"database_observability_query_hash_info",
	))
}
