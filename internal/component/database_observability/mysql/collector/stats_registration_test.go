package collector

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

// TestTableStatsAndIndexStatsShareRegistry guards against a regression where
// TableStats and IndexStats fail to both register on the single registry the
// component actually shares them on (e.g. if a future change makes them
// declare an identical metric descriptor again). Per-collector tests each use
// their own fresh registry and can't catch this; only registering both
// together, as the component does, can.
func TestTableStatsAndIndexStatsShareRegistry(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	logger := util.TestAlloyLogger(t).Slog()

	tableStats, err := NewTableStats(TableStatsArguments{DB: db, Registry: registry, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, tableStats.Start(t.Context()))
	defer tableStats.Stop()

	indexStats, err := NewIndexStats(IndexStatsArguments{DB: db, Registry: registry, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, indexStats.Start(t.Context()))
	defer indexStats.Stop()
}
