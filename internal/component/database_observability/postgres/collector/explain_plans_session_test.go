package collector

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
)

// sessionTestDriver is a database driver that records which connection ran each statement, and can
// break a connection at a given statement the way a dropped network connection does: with
// driver.ErrBadConn, which makes database/sql retry on a new connection.
type sessionTestDriver struct {
	mu sync.Mutex

	// failAtCall is the 1-based number of the statement that fails with driver.ErrBadConn, once.
	// Zero means no statement fails.
	failAtCall int

	calls      int
	failed     bool
	opened     int
	statements []sessionTestStatement
}

type sessionTestStatement struct {
	conn  int
	query string
}

// connIDs returns the connections that ran at least one statement, in the order they first did.
func (d *sessionTestDriver) connIDs() []int {
	d.mu.Lock()
	defer d.mu.Unlock()

	var ids []int
	seen := map[int]bool{}
	for _, s := range d.statements {
		if !seen[s.conn] {
			seen[s.conn] = true
			ids = append(ids, s.conn)
		}
	}
	return ids
}

func (d *sessionTestDriver) Open(string) (driver.Conn, error) { return d.newConn(), nil }

func (d *sessionTestDriver) newConn() *sessionTestConn {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.opened++
	return &sessionTestConn{d: d, id: d.opened}
}

type sessionTestConnector struct{ d *sessionTestDriver }

func (c sessionTestConnector) Connect(context.Context) (driver.Conn, error) {
	return c.d.newConn(), nil
}
func (c sessionTestConnector) Driver() driver.Driver { return c.d }

type sessionTestConn struct {
	d  *sessionTestDriver
	id int
}

func (c *sessionTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (c *sessionTestConn) Close() error                        { return nil }
func (c *sessionTestConn) Begin() (driver.Tx, error)           { return nil, errors.New("not used") }

func (c *sessionTestConn) record(query string) error {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()

	c.d.calls++
	if c.d.failAtCall == c.d.calls && !c.d.failed {
		c.d.failed = true
		return driver.ErrBadConn
	}
	c.d.statements = append(c.d.statements, sessionTestStatement{conn: c.id, query: query})
	return nil
}

func (c *sessionTestConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if err := c.record(query); err != nil {
		return nil, err
	}
	return driver.RowsAffected(0), nil
}

func (c *sessionTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if err := c.record(query); err != nil {
		return nil, err
	}
	return &sessionTestRows{plan: []byte(`[{"Plan":{"Node Type":"Result","Total Cost":1.0,"Plan Rows":1}}]`)}, nil
}

type sessionTestRows struct {
	plan []byte
	done bool
}

func (r *sessionTestRows) Columns() []string { return []string{"json"} }
func (r *sessionTestRows) Close() error      { return nil }
func (r *sessionTestRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.plan
	return nil
}

func newSessionTestExplainPlans(t *testing.T, d *sessionTestDriver) *ExplainPlans {
	t.Helper()

	version, err := semver.ParseTolerant("17.0")
	require.NoError(t, err)

	return &ExplainPlans{
		dbDSN:     "postgres://user:pass@host:1234/database",
		dbVersion: version,
		dbConnectionFactory: func(string) (*sql.DB, error) {
			return sql.OpenDB(sessionTestConnector{d: d}), nil
		},
		logger: logging.NewSlogNop(),
	}
}

func TestExplainPlansRunEveryStatementOnOneConnection(t *testing.T) {
	qi := queryInfo{datname: "testdb", queryId: "123456", queryText: "select * from some_table where id = $1", calls: 10, callsReset: time.Now()}

	t.Run("all the statements of both plans run on the same connection", func(t *testing.T) {
		d := &sessionTestDriver{}
		c := newSessionTestExplainPlans(t, d)

		plan, verbosePlan, err := c.fetchExplainPlanJSON(t.Context(), qi, true)
		require.NoError(t, err)
		require.NotEmpty(t, plan)
		require.NotEmpty(t, verbosePlan)

		require.Equal(t, []int{1}, d.connIDs())
		require.Equal(t, 1, d.opened, "only one connection is ever opened")
		require.Len(t, d.statements, 8, "search_path, PREPARE, plan_cache_mode, EXPLAIN, pruning off, DISCARD PLANS, verbose EXPLAIN, DEALLOCATE")
	})

	t.Run("a connection that breaks halfway does not move the rest to another connection", func(t *testing.T) {
		// The fourth statement is the regular EXPLAIN. database/sql retries a statement that fails
		// with driver.ErrBadConn on a new connection when it is not tied to one, which would run
		// the EXPLAIN in a session that has no prepared statement.
		d := &sessionTestDriver{failAtCall: 4}
		c := newSessionTestExplainPlans(t, d)

		_, _, err := c.fetchExplainPlanJSON(t.Context(), qi, true)
		require.Error(t, err)

		require.Equal(t, []int{1}, d.connIDs(), "no statement ran on another connection")
		require.Equal(t, 1, d.opened, "no second connection was opened")
	})

	t.Run("a connection that breaks during the verbose plan does not move the rest to another connection", func(t *testing.T) {
		// The seventh statement is the verbose EXPLAIN, after the pruning setting was turned off on
		// the first connection.
		d := &sessionTestDriver{failAtCall: 7}
		c := newSessionTestExplainPlans(t, d)

		plan, verbosePlan, err := c.fetchExplainPlanJSON(t.Context(), qi, true)
		require.NoError(t, err, "the regular plan is kept")
		require.NotEmpty(t, plan)
		require.Empty(t, verbosePlan)

		require.Equal(t, []int{1}, d.connIDs(), "no statement ran on another connection")
		require.Equal(t, 1, d.opened, "no second connection was opened")
	})
}
