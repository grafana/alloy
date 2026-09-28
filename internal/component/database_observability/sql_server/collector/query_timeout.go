package collector

import (
	"context"
	"time"
)

const DefaultQueryTimeout = 10 * time.Second

func queryTimeoutOrDefault(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultQueryTimeout
	}
	return timeout
}

func withQueryTimeout(ctx context.Context, timeout time.Duration, query func(context.Context) error) error {
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return query(queryCtx)
}
