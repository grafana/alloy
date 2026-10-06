package database_observability

import "database/sql"

// DefaultMaxOpenConnections is the default upper bound on the number of
// connections a database_observability component keeps open to one database.
const DefaultMaxOpenConnections = 15

// ConfigureConnectionPool limits the pool of db to at most maxOpen open
// connections. Idle connections are kept up to the same number so that the
// pool reuses connections instead of opening and closing them repeatedly.
// A maxOpen of zero or less leaves the pool unlimited, as in database/sql.
func ConfigureConnectionPool(db *sql.DB, maxOpen int) {
	if maxOpen <= 0 {
		return
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen)
}
