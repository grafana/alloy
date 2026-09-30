package collector

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/sqlfingerprint"
)

func TestQueryDetails_OTelDBQueryFingerprint(t *testing.T) {
	for _, tc := range []struct{ name, statistics, application string }{
		{`parameters`, `SELECT * FROM [dbo].[t] WHERE id = @p1`, `SELECT * FROM [dbo].[t] WHERE id = 42`},
		{`unsupported`, `EXEC dbo.p`, ``},
		{`multiple shapes`, `SELECT * FROM t; SELECT * FROM u`, ``},
		{`partial failure`, `SELECT * FROM t; START TRANSACTION`, ``},
		{`over limit`, strings.Repeat(" ", 1<<20) + "SELECT 1", ``},
		{`incomplete`, `SELECT * FROM t WHERE id =`, ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := loki.NewCollectingHandler()
			defer handler.Stop()
			c, err := NewQueryDetails(QueryDetailsArguments{
				EntryHandler: handler,
				Logger:       util.TestAlloyLogger(t).Slog(),
			})
			require.NoError(t, err)
			c.emit("db", "native-id", tc.statistics)

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
			result := f.Fingerprint(sqlfingerprint.SQLServer, tc.application)
			require.Empty(t, result.Failures)
			require.Len(t, result.Fingerprints, 1)
			fields := strings.SplitN(line, " otel_db_query_fingerprint=", 2)
			require.Len(t, fields, 2)
			require.Equal(t, `"`+result.Fingerprints[0]+`"`, fields[1])
		})
	}
}
