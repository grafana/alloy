package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.uber.org/atomic"
)

const (
	SetupActorsCollector = "setup_actors"

	selectUserQuery = `SELECT substring_index(current_user(), '@', 1)`

	selectQuery = `SELECT enabled, history
		FROM performance_schema.setup_actors
		WHERE user = ?`

	updateQuery = `UPDATE performance_schema.setup_actors
		SET enabled='NO', history='NO'
		WHERE user = ?`

	insertQuery = `INSERT INTO performance_schema.setup_actors
		(host, user, role, enabled, history)
		VALUES ('%', ?, '%', 'NO', 'NO')`
)

type SetupActorsArguments struct {
	DB                    *sql.DB
	Logger                *slog.Logger
	CollectInterval       time.Duration
	AutoUpdateSetupActors bool
}

type SetupActors struct {
	dbConnection          *sql.DB
	collectInterval       time.Duration
	autoUpdateSetupActors bool

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewSetupActors(args SetupActorsArguments) (*SetupActors, error) {
	return &SetupActors{
		dbConnection:          args.DB,
		running:               &atomic.Bool{},
		logger:                args.Logger.With("collector", SetupActorsCollector),
		collectInterval:       args.CollectInterval,
		autoUpdateSetupActors: args.AutoUpdateSetupActors,
	}, nil
}

func (c *SetupActors) Name() string {
	return SetupActorsCollector
}

func (c *SetupActors) Start(ctx context.Context) error {
	c.logger.Debug("collector started")
	c.running.Store(true)

	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	var user string
	if err := c.dbConnection.QueryRowContext(ctx, selectUserQuery).Scan(&user); err != nil {
		c.logger.Error("failed to get current user", "err", err)
		c.running.Store(false)
		cancel()
		return err
	}

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			if err := c.checkSetupActors(c.ctx, user); err != nil {
				c.logger.Error("collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	})

	return nil
}

func (c *SetupActors) Stopped() bool {
	return !c.running.Load()
}

func (c *SetupActors) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *SetupActors) checkSetupActors(ctx context.Context, user string) error {
	var enabled, history string
	err := c.dbConnection.QueryRowContext(ctx, selectQuery, user).Scan(&enabled, &history)
	if errors.Is(err, sql.ErrNoRows) {
		if c.autoUpdateSetupActors {
			return c.insertSetupActors(ctx, user)
		} else {
			c.logger.Info("setup_actors configuration missing, but auto-update is disabled")
			return nil
		}
	} else if err != nil {
		c.logger.Error("failed to query setup_actors table", "err", err)
		return err
	}

	if strings.ToUpper(enabled) != "NO" || strings.ToUpper(history) != "NO" {
		if c.autoUpdateSetupActors {
			return c.updateSetupActors(ctx, user, enabled, history)
		} else {
			c.logger.Info("setup_actors configuration is not correct, but auto-update is disabled")
			return nil
		}
	}

	return nil
}

func (c *SetupActors) insertSetupActors(ctx context.Context, user string) error {
	_, err := c.dbConnection.ExecContext(ctx, insertQuery, user)
	if err != nil {
		c.logger.Error("failed to insert setup_actors row", "err", err, "user", user)
		return err
	}

	c.logger.Debug("inserted new setup_actors row", "user", user)
	return nil
}

func (c *SetupActors) updateSetupActors(ctx context.Context, user string, enabled string, history string) error {
	r, err := c.dbConnection.ExecContext(ctx, updateQuery, user)
	if err != nil {
		c.logger.Error("failed to update setup_actors row", "err", err, "user", user)
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		c.logger.Error("failed to get rows affected from setup_actors update", "err", err)
		return err
	}
	if rowsAffected == 0 {
		c.logger.Error("no rows affected from setup_actors update", "user", user)
		return fmt.Errorf("no rows affected from setup_actors update")
	}

	c.logger.Debug("updated setup_actors row", "rows_affected", rowsAffected, "previous_enabled", enabled, "previous_history", history, "user", user)
	return nil
}
