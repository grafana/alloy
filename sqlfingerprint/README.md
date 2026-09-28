# SQL query fingerprints

`github.com/grafana/alloy/sqlfingerprint` is a public Go package shared by the
Alloy trace processor and database observability backends. It has only standard
library dependencies and doesn't connect to a database. No CGO is required.

Use identical library versions and options on both sides. For example, when a
user selects an expensive PostgreSQL query in your backend:

```go
normalizer, err := sqlfingerprint.New(sqlfingerprint.Options{
    PostgreSQLVersion: 18,
})
if err != nil {
    return err
}
result := normalizer.Fingerprint(sqlfingerprint.PostgreSQL, queryFromStatistics)
// Check result.Failures and search for each result.Fingerprints entry.
```

`Fingerprint` accepts original SQL or database-normalized text. It returns distinct
fingerprints in statement order, plus SQL-free failure reasons with zero-based
statement indexes. An empty fingerprint list means there is nothing to search for.
Never replace that result with a hash of the input or an error sentinel.

The Alloy component reads `db.query.text` (falling back to `db.statement`) and
`db.system.name` (falling back to `db.system`). It writes an array to
`db.query.fingerprint`, including when there is just one fingerprint. It does
not read statistics itself. See [the example configuration](../example/sql-fingerprint/config.alloy).

## Correlation contract

Fingerprints have the form `v5:<dialect>:<64 lowercase hex digits>`. SHA-256 hashes
a version and dialect prefix followed by tokens framed with a kind byte and a
big-endian, four-byte UTF-8 length. This avoids ambiguity between token boundaries.
The fingerprint protocol includes token kinds and normalization rules; changing
any equivalence rule requires a new version. It is independent of native database
query IDs, sessions, and plans.

Version 5 recognizes PostgreSQL named Python placeholders in value positions.
Fingerprints differ from earlier versions for all dialects, since the protocol
version is included in the hash. Use version 5 in both the
trace processor and the service that fingerprints database statistics.

Values and bind names are erased. SQL operators, aliases, explicit schema names,
and identifier case are retained, except PostgreSQL's unquoted name folding.
Consequently, `orders` and `public.orders` are different shapes. Literal types and
parameter identity aren't recovered from a normalized placeholder. Query shapes
are candidates for correlation, not proof of semantic or execution-plan equality.
Restrict trace searches by database instance, database name, and time range.

Rules for normalized list text follow these primary references:

| Database | Statistics text | Rules |
| --- | --- | --- |
| PostgreSQL | `pg_stat_statements.query` | `$n` parameters; preserve list lengths for 16/17; recognize the 18 constant `IN`/`ARRAY` list marker `$1 /*, ... */`. |
| MySQL | Performance Schema `DIGEST_TEXT` | `?` parameters; `?, ...` value lists; `IN (...)`; single/multiple-column rows; repeated-row comments. |
| Microsoft SQL Server | Query Store query text or a statement extracted from cached batch text | `@name` parameters, optional leading parameter type declarations, quoted identifiers, and unchanged list lengths. |

Sources: [PostgreSQL statistics normalization](https://www.postgresql.org/docs/18/pgstatstatements.html),
[MySQL statement digest documentation](https://dev.mysql.com/doc/refman/8.4/en/performance-schema-statement-digests.html),
[MySQL digest token display forms](https://github.com/mysql/mysql-server/blob/8.4/sql/gen_lex_token.cc),
[SQL Server query text](https://learn.microsoft.com/en-us/sql/relational-databases/system-catalog-views/sys-query-store-query-text-transact-sql),
and [SQL Server statement offsets](https://learn.microsoft.com/en-us/sql/relational-databases/system-dynamic-management-views/sys-dm-exec-query-stats-transact-sql).
The Go implementation is independent; it doesn't embed database server source.

## Coverage and limits

For MySQL, a row of multiple values such as `VALUES (?,?,?,?,?,?)` matches the
native `VALUES (...)` marker. Simple `INSERT [IGNORE] [INTO] table (columns)`
statements sort their explicit column list when every row collapses to a constant
value marker. The table can be database-qualified. For example,
`INSERT INTO t (b,a) VALUES (?,?)` matches `INSERT INTO t (a,b) VALUES (...)`.
This deliberately groups inserts across different column orders, including those
produced by different application frameworks.

Column names, table qualification, and INSERT modifiers remain significant.
Single rows and repeated rows remain distinct. Expressions, `DEFAULT`,
`INSERT ... SELECT`, row aliases, trailing update clauses, and other INSERT
forms retain column order so that expression-to-column assignments stay intact.

Ordinary identifiers match their backtick-quoted digest form. The nonreserved
[COMMENT keyword](https://dev.mysql.com/doc/refman/8.4/en/keywords.html) can name
a column without quotes and still appears as uppercase `COMMENT` in digest text;
both keyword spellings share one token. Quoted and qualified identifiers retain
their spelling and case. These rules make the Java `reviews` insert and its
MySQL digest in `mysql_digest_test.go` produce the same fingerprint.

The PostgreSQL adapter recognizes the named Python `%(name)s` form in expression
value positions, using the [Psycopg parameter syntax](https://www.psycopg.org/psycopg3/docs/basic/params.html)
generated by SQLAlchemy. For example, `%(primary_keys_1)s::INTEGER` and
`$1::INTEGER` enter the same cast and list normalization rules. Parameter names
are erased; PostgreSQL 16/17 list lengths and PostgreSQL 18 constant-list collapse
continue to follow the configured version. Quoted strings, quoted identifiers,
and native modulo expressions retain their existing handling. Malformed named
placeholders fail parsing, and an unterminated placeholder stops the batch.

The PostgreSQL adapter treats any final standalone `?` token as an obfuscated
trailing comment, such as Rails query tags, regardless of the preceding clause.
For example, both `ORDER BY name ASC ?` and `SELECT * FROM restaurants ?` match
those statements followed by `/* comment */`. Question marks in value positions
are normalized as parameters even without a trailing comment: Java's
`WHERE ?=? ORDER BY name` matches `WHERE $1=$2 ORDER BY name`.
After removing a comment marker, `= ? ?` also matches `= $1 /* comment */`.
Quoted question marks and infix `?`, `?|`, and `?&` JSON operators are preserved.
The remaining statement must still pass validation. This assumes a terminal `?`
is a comment marker, not a missing value or an incomplete JSON operator expression.
Ruby's [SQL obfuscator](https://github.com/open-telemetry/opentelemetry-ruby-contrib/blob/main/helpers/sql-processor/lib/opentelemetry/helpers/sql_processor/obfuscator.rb)
replaces both literal values and comments with question marks.

This experimental implementation uses a dialect-aware lexer and structural
normalizer. It is **not a complete port of the three database SQL grammars** or
their catalog-dependent normalizers. It supports standalone `SELECT`, `INSERT`,
`UPDATE`, `DELETE`, `MERGE`, `WITH`, and `VALUES` statements and semicolon-separated
batches. The fixtures test representative query/statistics pairs, not complete
server compatibility. The target matrix is PostgreSQL 16/17/18, MySQL 8.4/9.7/26.7,
and SQL Server 2019/2022/2025. Representative text was checked against local
PostgreSQL 16/18 and MySQL 8.0/8.4 servers, and retained in `native_test.go`.
A full live-server conformance matrix remains necessary before claiming complete
coverage of the target versions; SQL Server currently has fixture tests only.

Stored procedures, procedural/transaction blocks, execution wrappers, DDL,
client `GO`/`DELIMITER` commands, optimizer hints, executable comments, Unicode
escape identifiers, and semicolon-free batches aren't supported. Procedural
batches stop at the first unsafe boundary, retaining earlier successful statements.
Unterminated strings/comments also stop the batch. Other recognized failures skip
the affected statement. The library checks lexical and structural validity, not
whether SQL would execute successfully on a server.

The component bounds a query to 1 MiB by default, 65,536 tokens per statement,
128 nested delimiters, and 256 nonempty statements per batch. It skips input over
the byte limit instead of hashing a prefix. Detected truncation is rejected;
truncation that still looks like a complete statement can't be detected from text
alone. Supply full statistics text, not UI previews, query summaries, or text
obfuscated by an unrelated algorithm. Extract the individual expensive statement
before fingerprinting a SQL Server cached batch, or every statement in the batch
becomes a search candidate.

Options cover PostgreSQL standard string escapes, MySQL `ANSI_QUOTES` and
`NO_BACKSLASH_ESCAPES`, and SQL Server `QUOTED_IDENTIFIER`. Other session-mode
changes aren't inferred. Route sources with different modes or PostgreSQL major
versions through separately configured processors, and retain the same options
in the backend.

## Trace lookup

For an array attribute, Tempo can test element equality:

```traceql
{ span.db.query.fingerprint = "v5:postgresql:<HASH>" }
```

Replace `<HASH>` with the hash returned by this package. Use an array-capable
Tempo storage format (vParquet4 or newer); see [Tempo array queries](https://grafana.com/docs/tempo/latest/traceql/construct-traceql-queries/#find-traces-with-arrays).
The processor doesn't configure Tempo indexing or retention, and sampled-out
traces won't be found.

## Verification

```shell
go test -race -tags=nodocker ./sqlfingerprint ./internal/component/otelcol/processor/sqlfingerprint
go test ./sqlfingerprint -run='^$' -fuzz=FuzzFingerprint -fuzztime=30s
```

Tests include native normalized-text markers, cases that must remain distinct,
partial batches, quoting modes, limits, concurrent reuse, processor reloads,
attribute precedence, downstream failures, and source-data preservation.
