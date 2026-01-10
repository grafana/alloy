package collector

import (
	"database/sql"
	"errors"
	"regexp"
)

type databaseConnectionFactory func(dsn string) (*sql.DB, error)

var (
	dsnParseRegex              = regexp.MustCompile(`^(\w+:\/\/.+\/)(?<dbname>[\w\-_\$]+)(\??.*$)`)
	defaultDbConnectionFactory = func(dsn string) (*sql.DB, error) {
		return sql.Open("postgres", dsn)
	}
)

// replaceDatabaseNameInDSN safely replaces the database name in a PostgreSQL DSN
// using regex to ensure only the database name portion is replaced, not other occurrences
func replaceDatabaseNameInDSN(dsn, newDatabaseName string) (string, error) {
	// Use the same regex pattern as in NewExplainPlan to find the database name
	matches := dsnParseRegex.FindStringSubmatch(dsn)

	if len(matches) < 4 {
		return "", errors.New("failed to parse DSN for database name replacement")
	}

	// Reconstruct the DSN with the new database name
	// matches[1] = prefix (protocol://user:pass@host:port/)
	// matches[2] = original database name (captured group)
	// matches[3] = suffix (query parameters)
	newDSN := matches[1] + newDatabaseName + matches[3]
	return newDSN, nil
}
