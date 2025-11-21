package prometheus

import (
	"testing"

	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/prometheus/storage"

	"github.com/stretchr/testify/require"
)

func TestRollback(t *testing.T) {
	ls := labelstore.New(nil, prometheus.DefaultRegisterer)
	fanout := NewFanout([]storage.Appendable{NewFanout(nil, prometheus.DefaultRegisterer, ls)}, prometheus.DefaultRegisterer, ls)
	app := fanout.Appender(t.Context())
	err := app.Rollback()
	require.NoError(t, err)
}

func TestCommit(t *testing.T) {
	ls := labelstore.New(nil, prometheus.DefaultRegisterer)
	fanout := NewFanout([]storage.Appendable{NewFanout(nil, prometheus.DefaultRegisterer, ls)}, prometheus.DefaultRegisterer, ls)
	app := fanout.Appender(t.Context())
	err := app.Commit()
	require.NoError(t, err)
}
