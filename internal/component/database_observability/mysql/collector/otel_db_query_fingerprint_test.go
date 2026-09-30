package collector

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/sqlfingerprint"
)

func TestQueryDetails_OTelDBQueryFingerprint(t *testing.T) {
	for _, tc := range []struct{ name, statistics, application string }{
		{`native digest`, "SELECT * FROM `t` WHERE `id` IN (...)", `SELECT * FROM t WHERE id IN (1,2,3)`},
		{`unsupported`, `SELECT /*+ hint */ * FROM t`, ``},
		{`multiple shapes`, `SELECT * FROM t; SELECT * FROM u`, ``},
		{`partial failure`, `SELECT * FROM t; START TRANSACTION`, ``},
		{`over limit`, strings.Repeat(" ", 1<<20) + "SELECT 1", ``},
		{`incomplete`, `SELECT * FROM t WHERE id =`, ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := loki.NewCollectingHandler()
			defer handler.Stop()
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()
			c, err := NewQueryDetails(QueryDetailsArguments{
				EntryHandler:    handler,
				Logger:          util.TestAlloyLogger(t).Slog(),
				DB:              db,
				StatementsLimit: 10,
			})
			require.NoError(t, err)
			mock.ExpectQuery(fmt.Sprintf(selectQueryTablesSamples, buildExcludedSchemasClause(nil), 10)).
				RowsWillBeClosed().WillReturnRows(sqlmock.NewRows([]string{"digest", "digest_text", "schema_name", "query_sample_text"}).AddRow("native-id", tc.statistics, "db", "SELECT * FROM unrelated_sample"))
			require.NoError(t, c.tablesFromEventsStatements(t.Context()))
			require.NoError(t, mock.ExpectationsWereMet())

			var line string
			require.Eventually(t, func() bool {
				for _, entry := range handler.Received() {
					if entry.Labels["op"] == database_observability.OP_QUERY_ASSOCIATION {
						line = entry.Line
						return true
					}
				}
				return false
			}, time.Second, time.Millisecond)
			require.Contains(t, line, "native-id")
			if tc.application == "" {
				require.NotContains(t, line, "otel_db_query_fingerprint=")
				return
			}
			// Compare the emitted statistics fingerprint with the trace-side input.
			f, err := sqlfingerprint.New(sqlfingerprint.Options{})
			require.NoError(t, err)
			result := f.Fingerprint(sqlfingerprint.MySQL, tc.application)
			require.Empty(t, result.Failures)
			require.Len(t, result.Fingerprints, 1)
			fields := strings.SplitN(line, " otel_db_query_fingerprint=", 2)
			require.Len(t, fields, 2)
			require.Equal(t, `"`+result.Fingerprints[0]+`"`, fields[1])
		})
	}
}
