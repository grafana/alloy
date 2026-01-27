package collector

import (
	"testing"
	"time"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueryHashRegistry_BasicOperations tests the basic functionality of the registry
func TestQueryHashRegistry_BasicOperations(t *testing.T) {
	registry := NewQueryHashRegistry(100, time.Hour)

	// Test Set and Get
	registry.Set("123", "hash123", "testdb")
	info, ok := registry.Get("123")
	require.True(t, ok)
	assert.Equal(t, "hash123", info.QueryHash)
	assert.Equal(t, "testdb", info.DatabaseName)

	// Test non-existent key
	_, ok = registry.Get("nonexistent")
	assert.False(t, ok)

	// Test GetAll
	registry.Set("456", "hash456", "testdb2")
	all := registry.GetAll()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "123")
	assert.Contains(t, all, "456")
}

// TestTraceparentAndCommentsIgnored validates that different traceparents and comments
// don't affect the fingerprint - critical for matching queries across different executions
func TestTraceparentAndCommentsIgnored(t *testing.T) {
	tests := []struct {
		name    string
		query1  string
		query2  string
		wantMsg string
	}{
		{
			name: "traceparent comments",
			query1: `UPDATE reading_lists 
				SET next_process_time = NOW() + INTERVAL $1
				WHERE id IN (SELECT id FROM target_lists) 
				/*traceparent='00-4cb5d529e7b69d28146d01f498d0d0f5-e12e843bcb93ea01-01'*/`,
			query2: `UPDATE reading_lists 
				SET next_process_time = NOW() + INTERVAL $1
				WHERE id IN (SELECT id FROM target_lists) 
				/*traceparent='00-90b56ade740cadac523c6b189eeab23c-bf4006d1336bec8f-01'*/`,
			wantMsg: "Different traceparents should produce same fingerprint",
		},
		{
			name:    "different comment types",
			query1:  `SELECT * FROM books WHERE stock > $1 -- single line comment`,
			query2:  `SELECT * FROM books WHERE stock > $1 /* block comment */`,
			wantMsg: "Different comment styles should produce same fingerprint",
		},
		{
			name:    "no comments vs with comments",
			query1:  `INSERT INTO rentals (book_id, user_id) VALUES ($1, $2)`,
			query2:  `INSERT INTO rentals (book_id, user_id) VALUES ($1, $2) /* some comment */`,
			wantMsg: "Query with and without comments should match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err := pg_query.Fingerprint(tt.query1)
			require.NoError(t, err)

			hash2, err := pg_query.Fingerprint(tt.query2)
			require.NoError(t, err)

			assert.Equal(t, hash1, hash2, tt.wantMsg)
			t.Logf("Fingerprint: %s", hash1)
		})
	}
}

// TestComplexQueryWithLiterals validates that complex production queries with literal values
// match their parameterized counterparts in pg_stat_statements
func TestComplexQueryWithLiterals(t *testing.T) {
	// Real complex CTE query from logs with LITERAL values
	logQueryWithLiterals := `
		WITH updated_rentals AS (
			UPDATE rentals r
			SET status = 'OVERDUE'
			WHERE id IN (
				SELECT id
				FROM rentals
				WHERE status = 'ACTIVE'
					AND expected_return_date < CURRENT_TIMESTAMP - INTERVAL '7 days'
				ORDER BY expected_return_date ASC
				LIMIT 1000
			)
			RETURNING id
		)
		SELECT COUNT(*) FROM updated_rentals`

	// Same query in pg_stat_statements with PARAMETERS
	pgStatQueryParameterized := `
		WITH updated_rentals AS (
			UPDATE rentals r
			SET status = $1
			WHERE id IN (
				SELECT id
				FROM rentals
				WHERE status = $2
					AND expected_return_date < CURRENT_TIMESTAMP - INTERVAL $3
				ORDER BY expected_return_date ASC
				LIMIT $4
			)
			RETURNING id
		)
		SELECT COUNT(*) FROM updated_rentals`

	logHash, err := pg_query.Fingerprint(logQueryWithLiterals)
	require.NoError(t, err)

	pgStatHash, err := pg_query.Fingerprint(pgStatQueryParameterized)
	require.NoError(t, err)

	assert.Equal(t, logHash, pgStatHash, "Complex query with literals must match parameterized version")
	t.Logf("Complex CTE fingerprint: %s", logHash)

	// Test the MOST complex production query (1257 chars) with literals
	complexInsertWithLiterals := `
		WITH new_books AS (
			INSERT INTO books (title, isbn, publication_date, rental_price_per_day, stock, category_id)
			SELECT
				CONCAT(t.word, ' ', adj.word) as title,
				CAST('978-' || ROW_NUMBER() OVER () AS TEXT) as isbn,
				CURRENT_DATE - (random() * 365 * INTERVAL '1 day') as publication_date,
				(5.00 + floor(random() * 20))::decimal(10,2) as rental_price_per_day,
				CASE
					WHEN random() < 0.2 THEN (50 + floor(random() * 50))
					ELSE (5 + floor(random() * 15))
				END as stock,
				(1 + floor(random() * 10))::integer as category_id
			FROM gen_themes t
			CROSS JOIN gen_adjectives adj
			ORDER BY random()
			LIMIT 100
			RETURNING id
		)
		SELECT DISTINCT nb.id FROM new_books nb`

	complexInsertParameterized := `
		WITH new_books AS (
			INSERT INTO books (title, isbn, publication_date, rental_price_per_day, stock, category_id)
			SELECT
				CONCAT(t.word, $1, adj.word) as title,
				CAST($2 || ROW_NUMBER() OVER () AS TEXT) as isbn,
				CURRENT_DATE - (random() * $3 * INTERVAL $4) as publication_date,
				($5 + floor(random() * $6))::decimal(10,2) as rental_price_per_day,
				CASE
					WHEN random() < $7 THEN ($8 + floor(random() * $9))
					ELSE ($10 + floor(random() * $11))
				END as stock,
				($12 + floor(random() * $13))::integer as category_id
			FROM gen_themes t
			CROSS JOIN gen_adjectives adj
			ORDER BY random()
			LIMIT $14
			RETURNING id
		)
		SELECT DISTINCT nb.id FROM new_books nb`

	complexLiteralHash, err := pg_query.Fingerprint(complexInsertWithLiterals)
	require.NoError(t, err)

	complexParamHash, err := pg_query.Fingerprint(complexInsertParameterized)
	require.NoError(t, err)

	assert.Equal(t, complexLiteralHash, complexParamHash, "Most complex query with literals must match parameterized version")
	t.Logf("Complex INSERT (14 params) fingerprint: %s", complexParamHash)
}

// TestDifferentArgumentTypes validates that different data types normalize correctly
// Testing: text, numbers (int/decimal), dates, booleans, intervals, arrays, NULL
func TestDifferentArgumentTypes(t *testing.T) {
	tests := []struct {
		name             string
		queryWithLiteral string
		queryWithParam   string
		description      string
	}{
		{
			name:             "integers",
			queryWithLiteral: `SELECT * FROM books WHERE category_id = 5 AND stock > 10`,
			queryWithParam:   `SELECT * FROM books WHERE category_id = $1 AND stock > $2`,
			description:      "Integer literals",
		},
		{
			name:             "decimals",
			queryWithLiteral: `SELECT * FROM books WHERE rental_price_per_day < 25.50`,
			queryWithParam:   `SELECT * FROM books WHERE rental_price_per_day < $1`,
			description:      "Decimal/numeric literals",
		},
		{
			name:             "strings",
			queryWithLiteral: `SELECT * FROM books WHERE title = 'Harry Potter' AND isbn LIKE '978-%'`,
			queryWithParam:   `SELECT * FROM books WHERE title = $1 AND isbn LIKE $2`,
			description:      "String literals",
		},
		{
			name:             "dates and timestamps",
			queryWithLiteral: `SELECT * FROM rentals WHERE rental_date >= '2024-01-01' AND rental_date < '2024-12-31'`,
			queryWithParam:   `SELECT * FROM rentals WHERE rental_date >= $1 AND rental_date < $2`,
			description:      "Date literals",
		},
		{
			name:             "booleans",
			queryWithLiteral: `SELECT * FROM books WHERE available = true AND featured = false`,
			queryWithParam:   `SELECT * FROM books WHERE available = $1 AND featured = $2`,
			description:      "Boolean literals",
		},
		{
			name:             "intervals",
			queryWithLiteral: `SELECT * FROM rentals WHERE expected_return_date < NOW() - INTERVAL '7 days'`,
			queryWithParam:   `SELECT * FROM rentals WHERE expected_return_date < NOW() - INTERVAL $1`,
			description:      "Interval literals",
		},
		{
			name:             "NULL values",
			queryWithLiteral: `SELECT * FROM books WHERE description IS NULL OR notes IS NOT NULL`,
			queryWithParam:   `SELECT * FROM books WHERE description IS NULL OR notes IS NOT NULL`,
			description:      "NULL literals (no parameterization)",
		},
		{
			name:             "type casts",
			queryWithLiteral: `UPDATE rentals SET status = 'OVERDUE'::rental_status WHERE id = 123`,
			queryWithParam:   `UPDATE rentals SET status = $1::rental_status WHERE id = $2`,
			description:      "Type-casted literals",
		},
		{
			name: "mixed types",
			queryWithLiteral: `
				SELECT * FROM rentals 
				WHERE user_id = 42 
					AND status = 'ACTIVE' 
					AND rental_date >= '2024-01-01'
					AND late_fee > 5.00
					AND expected_return_date < NOW() - INTERVAL '1 week'`,
			queryWithParam: `
				SELECT * FROM rentals 
				WHERE user_id = $1 
					AND status = $2 
					AND rental_date >= $3
					AND late_fee > $4
					AND expected_return_date < NOW() - INTERVAL $5`,
			description: "Multiple different types in one query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			literalHash, err := pg_query.Fingerprint(tt.queryWithLiteral)
			require.NoError(t, err, "Failed to fingerprint query with literals")

			paramHash, err := pg_query.Fingerprint(tt.queryWithParam)
			require.NoError(t, err, "Failed to fingerprint query with parameters")

			assert.Equal(t, literalHash, paramHash,
				"Fingerprints must match for %s", tt.description)

			t.Logf("%s - Fingerprint: %s", tt.description, literalHash)
		})
	}
}
