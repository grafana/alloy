package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// maxLockTimeout caps how long a collector query waits for a lock held by
// someone else, such as a long DDL statement on a table it reads.
const maxLockTimeout = 5 * time.Second

// inTransactionWithTimeouts runs fn in a read-only transaction in which the
// server itself bounds every statement: statement_timeout ends a statement
// that runs longer than timeout, and lock_timeout ends one that waits for a
// lock. Unlike cancelling the client's context, both still apply if the client
// goes away. They are set with SET LOCAL, so they end with the transaction and
// never leak to the connection's next user.
func inTransactionWithTimeouts(ctx context.Context, conn *sql.DB, timeout time.Duration, fn func(ctx context.Context, tx *sql.Tx) error) error {
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // the transaction is read-only; a failed rollback after commit is expected

	lockTimeout := min(timeout, maxLockTimeout)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d; SET LOCAL lock_timeout = %d",
		timeout.Milliseconds(), lockTimeout.Milliseconds())); err != nil {
		return fmt.Errorf("set timeouts: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
