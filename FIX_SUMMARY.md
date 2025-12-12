# Fix Summary: OPERATOR_pg_catalog_ Spurious Field

## Problem

The collector was generating a spurious `OPERATOR_pg_catalog_` field with value `")"` when processing PostgreSQL error logs that contained context fields with complex SQL statements.

### Example Log Line

```
context="SQL statement "SELECT 1 FROM ONLY "public"."books" x WHERE "id" OPERATOR(pg_catalog.=) $1 FOR KEY SHARE OF x""
```

### Root Cause

The `emitToLoki` function in `error_logs.go` was building logfmt-style output by directly interpolating string values into formatted strings without proper escaping:

```go
// BROKEN - Before fix
if parsed.Context != "" {
    logMessage += fmt.Sprintf(` context="%s"`, parsed.Context)
}
```

When the context field contained embedded double quotes (like `"public"."books"`), the resulting output had unescaped quotes:

```
context="SQL statement "SELECT 1 FROM ONLY "public"."books" x WHERE "id" OPERATOR(pg_catalog.=) $1 FOR KEY SHARE OF x""
```

Logfmt parsers would see the first embedded quote after `"SQL statement ` and interpret it as the end of the context value. The parser would then try to parse the remainder as new key-value pairs:

```
"SELECT 1 FROM ONLY "public"."books" x WHERE "id" OPERATOR(pg_catalog.=) ...
```

When it encountered `OPERATOR(pg_catalog.=)`, it would parse:
- Key: `OPERATOR(pg_catalog.` (after converting `.` to `_`)
- Separator: `=`
- Value: `)`

This created the spurious `OPERATOR_pg_catalog_=")"` field.

## Solution

Use Go's `strconv.Quote()` function to properly escape all string values before embedding them in the logfmt output. `strconv.Quote()` handles all necessary escaping including:

- Double quotes → `\"`
- Backslashes → `\\`
- Newlines → `\n`
- Tabs → `\t`
- Carriage returns → `\r`

### Changes Made

1. **Added import** to `error_logs.go`:
   ```go
   import "strconv"
   ```

2. **Updated all string formatting** in `emitToLoki()`:
   ```go
   // FIXED - After fix
   if parsed.Context != "" {
       logMessage += fmt.Sprintf(` context=%s`, strconv.Quote(parsed.Context))
   }
   ```

   Note: `strconv.Quote()` returns the value with surrounding quotes, so we don't add them manually in the format string.

3. **Applied to all string fields**:
   - severity, datname, user, backend_type, message
   - sqlstate, sqlstate_class, sqlstate_class_code
   - session_id, app, client
   - table, constraint, constraint_type, column
   - detail, hint, context, statement
   - lock_type, timeout_type, tuple_location
   - blocked_query, blocker_query
   - function, auth_method, hba_line

### Example Output After Fix

```
context="SQL statement \"SELECT 1 FROM ONLY \"public\".\"books\" x WHERE \"id\" OPERATOR(pg_catalog.=) $1 FOR KEY SHARE OF x\""
```

Now the embedded quotes are properly escaped with backslashes, so logfmt parsers will correctly identify the entire value as belonging to the `context` field, and no spurious fields will be created.

## Tests Added

1. **`TestErrorLogsCollector_StrconvQuote`**: Unit test verifying `strconv.Quote()` behavior with various inputs including the complex SQL case.

2. **`TestErrorLogsCollector_ContextWithOperatorFunction`**: Integration test that:
   - Processes the actual problematic log line
   - Verifies no spurious `OPERATOR_pg_catalog_` field is created
   - Confirms the context field is properly quoted and escaped
   - Validates the OPERATOR function is preserved in the output

## Files Modified

- `internal/component/database_observability/postgres/collector/error_logs.go`
- `internal/component/database_observability/postgres/collector/error_logs_test.go`
