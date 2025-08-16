package collector

import (
	"fmt"

	"github.com/DataDog/go-sqllexer"
)

// extractTableNames extracts the table names from a SQL query
func extractTableNames(sql string) ([]string, error) {
	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(true),
	)
	_, metadata, err := normalizer.Normalize(sql, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	if err != nil {
		return nil, fmt.Errorf("failed to normalize SQL: %w", err)
	}

	// Return all table names, including those that end with "..." for truncated queries, as we can't know if the table name was truncated or not
	return metadata.Tables, nil
}

// redact obfuscates a SQL query by replacing literals with ? placeholders
func redact(sql string) string {
	obfuscatedSql := sqllexer.NewObfuscator().Obfuscate(sql)
	return obfuscatedSql
}
