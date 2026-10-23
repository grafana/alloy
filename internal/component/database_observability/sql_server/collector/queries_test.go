package collector

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryMarkers(t *testing.T) {
	// Keep this explicit list in sync with queries.go. Query builders are covered
	// below with representative inputs; this suite does not discover SQL automatically.
	for name, query := range map[string]string{
		"ping":              PingQuery,
		"server info":       SelectServerInfo,
		"original login":    SelectOriginalLogin,
		"metrics":           selectQueryMetrics,
		"query store state": selectQueryStoreState,
		"permissions":       selectMyPermissionsQuery,
		"query store rows":  selectQueryStoreHasRowsQuery,
		"databases":         fmt.Sprintf(selectDatabasesTemplate, "('master', 'model')"),
		"tables":            fmt.Sprintf(selectTablesTemplate, "('sys', 'information_schema')"),
		"columns":           fmt.Sprintf(selectColumnsTemplate, "('table', 'quote''s table')"),
		"indexes":           fmt.Sprintf(selectIndexesTemplate, "('table', 'quote''s table')"),
		"foreign keys":      fmt.Sprintf(selectForeignKeysTemplate, "('table', 'quote''s table')"),
		"use database":      fmt.Sprintf(useDatabaseTemplate, "[SELECT odd]]name]"),
	} {
		t.Run(name, func(t *testing.T) { requireQueryMarker(t, query) })
	}
}

func TestQueryBuilderMarkers(t *testing.T) {
	for _, exclude := range []bool{false, true} {
		for _, hashes := range [][]string{{testHash}, {testHash, "aabbccddeeff0011"}} {
			for name, build := range map[string]func() (string, []any, error){
				"details":                         func() (string, []any, error) { return buildQueryTextStatement(hashes, exclude) },
				"plans":                           func() (string, []any, error) { return buildExplainPlansStatement(hashes, exclude) },
				"samples without user exclusions": func() (string, []any, error) { return buildQuerySamplesStatement(hashes, nil, exclude) },
				"samples with user exclusions": func() (string, []any, error) {
					return buildQuerySamplesStatement(hashes, []string{"db-o11y", "quote's login"}, exclude)
				},
			} {
				t.Run(fmt.Sprintf("%s/exclude=%t/hashes=%d", name, exclude, len(hashes)), func(t *testing.T) {
					query, _, err := build()
					require.NoError(t, err)
					requireQueryMarker(t, query)
				})
			}
		}
	}
}

// requireQueryMarker checks that each query contains exactly one QueryMarker
// immediately after a SELECT or USE.
func requireQueryMarker(t *testing.T, query string) {
	t.Helper()

	require.Equal(t, 1, strings.Count(query, QueryMarker), "collector query marker must appear exactly once")

	beforeMarker, _, found := strings.Cut(query, QueryMarker)
	require.True(t, found, "missing collector query marker")

	fields := strings.Fields(beforeMarker)
	require.NotEmpty(t, fields, "collector query marker must follow SELECT or USE")

	command := fields[len(fields)-1]
	require.True(t, strings.EqualFold(command, "SELECT") || strings.EqualFold(command, "USE"), "collector query marker must follow SELECT or USE")
}
