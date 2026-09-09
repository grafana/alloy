package collector

import (
	"context"
	"database/sql"
	"fmt"
)

// discoverDatabases lists the databases on the Postgres instance that the
// current connection is allowed to CONNECT to, via the pg_database catalog
// view (readable from any single connection). Callers use this to fan out
// per-database connections, since most stat views (e.g. pg_stat_user_tables,
// pg_stat_user_indexes) only ever report on the database a connection is
// actually established to.
func discoverDatabases(ctx context.Context, conn *sql.DB, excludeDatabases []string) ([]string, error) {
	query := fmt.Sprintf(selectAllDatabases, buildExcludedDatabasesClause(excludeDatabases))
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to discover databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var datname string
		if err := rows.Scan(&datname); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}
		databases = append(databases, datname)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}

	return databases, nil
}

// connectToDatabase opens a connection to dbName by rewriting the database
// name in dsn, using factory. If dbName is the database dsn (and so initial)
// already points to, initial is reused directly instead of opening a
// redundant connection: with the real sql.Open-based factory, a freshly
// opened *sql.DB is never pointer-equal to initial even for an identical
// DSN, so skipping the redundant open/close has to happen here, up front.
// The returned closeFn closes the connection unless it is initial (in which
// case closing it is the caller's responsibility elsewhere).
func connectToDatabase(dsn, dbName string, factory databaseConnectionFactory, initial *sql.DB) (conn *sql.DB, closeFn func(), err error) {
	noopClose := func() {}

	if currentDBName, err := databaseNameFromDSN(dsn); err == nil && currentDBName == dbName {
		return initial, noopClose, nil
	}

	databaseDSN, err := replaceDatabaseNameInDSN(dsn, dbName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create DSN for database %s: %w", dbName, err)
	}

	conn, err = factory(databaseDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create connection to database %s: %w", dbName, err)
	}

	closeFn = func() {
		if conn != initial {
			conn.Close()
		}
	}

	return conn, closeFn, nil
}
