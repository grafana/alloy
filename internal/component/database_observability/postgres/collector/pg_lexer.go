package collector

import (
	"fmt"
	"strings"

	"github.com/DataDog/go-sqllexer"
)

// ExtractTableNames extracts the table names from a SQL query
func ExtractTableNames(sql string) ([]string, error) {
	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(true),
	)
	_, metadata, err := normalizer.Normalize(sql, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	if err != nil {
		return nil, fmt.Errorf("failed to normalize SQL: %w", err)
	}

	tables := make([]string, 0)

	for _, table := range metadata.Tables {
		if !strings.HasSuffix(table, "...") {
			tables = append(tables, table)
		}
	}

	return tables, nil
}

// Redact obfuscates a SQL query by replacing literals with ? placeholders
func Redact(sql string) string {
	obfuscatedSql := sqllexer.NewObfuscator().Obfuscate(sql)
	return obfuscatedSql
}
