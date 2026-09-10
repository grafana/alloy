package collector

import (
	"context"
	"database/sql"
	"fmt"
)

// discoverDatabases lists databases the current connection can reach, via
// pg_database (readable from any single connection) -- used to fan out
// per-database connections, since most stat views only report on the
// database a connection is actually established to.
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

// connectToDatabase opens a connection to dbName by rewriting dsn. If dbName
// is already what dsn (and so initial) points to, it reuses initial instead
// of opening a redundant connection -- sql.Open never returns something
// pointer-equal to an existing *sql.DB, so this has to be checked by name up
// front, not via "conn != initial" after the fact. closeFn closes the
// connection unless it's initial.
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
