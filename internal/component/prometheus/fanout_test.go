package prometheus

import (
	"testing"

	"github.com/grafana/agent/internal/service/labelstore"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/prometheus/storage"

	"context"

	"github.com/stretchr/testify/require"
)

func TestRollback(t *testing.T) {
	ls := labelstore.New(nil, prometheus.DefaultRegisterer)
	fanout := NewFanout([]storage.Appendable{NewFanout(nil, "1", prometheus.DefaultRegisterer, ls)}, "", prometheus.DefaultRegisterer, ls)
	app := fanout.Appender(context.Background())
	err := app.Rollback()
	require.NoError(t, err)
}

func TestCommit(t *testing.T) {
	ls := labelstore.New(nil, prometheus.DefaultRegisterer)
	fanout := NewFanout([]storage.Appendable{NewFanout(nil, "1", prometheus.DefaultRegisterer, ls)}, "", prometheus.DefaultRegisterer, ls)
	app := fanout.Appender(context.Background())
	err := app.Commit()
	require.NoError(t, err)
}
