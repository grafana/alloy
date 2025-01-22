package mysql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax"
)

type noopLogger struct{}

func (d *noopLogger) Log(_ ...interface{}) error {
	return nil
}

type querierMock struct {
	mock.Mock
}

func (q *querierMock) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	//TODO implement me
	panic("implement me")
}

func (q *querierMock) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	arguments := q.Called(ctx, query)
	return arguments.Get(0).(*sql.Rows), arguments.Error(1)
}

func (q *querierMock) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	//TODO implement me
	panic("implement me")
}

func Test_Fluffles(t *testing.T) {
	q := &querierMock{}
	q.On("QueryContext", mock.Anything, mock.Anything, mock.Anything).Return(&sql.Rows{}, nil)

	var exampleAlloyConfig = `
		data_source_name = "root:secret_password@tcp(localhost:3306)/mydb"
		forward_to = []
		enable_collectors = ["collector1"]
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	c := &Component{
		args:     args,
		opts:     component.Options{Logger: &noopLogger{}},
		registry: prometheus.NewRegistry(),
	}
	//require.NoError(t, c.startCollectors())

	//c := &Component{
	//	args:     Arguments{CollectInterval: time.Second},
	//	opts:     component.Options{Logger: &noopLogger{}},
	//	registry: prometheus.NewRegistry(),
	//}
	err = theRealStartCollectors(q, c, nil)
	require.NoError(t, err)

	require.NotNil(t, c.args.EnableCollectors)
	require.Equal(t, []string{"collector1"}, c.args.EnableCollectors)
}
