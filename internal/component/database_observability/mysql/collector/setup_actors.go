package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/runtime/logging/level"
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
	Logger                log.Logger
	CollectInterval       time.Duration
	AutoUpdateSetupActors bool
}

type SetupActors struct {
	dbConnection          *sql.DB
	collectInterval       time.Duration
	autoUpdateSetupActors bool

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewSetupActors(args SetupActorsArguments) (*SetupActors, error) {
	return &SetupActors{
		dbConnection:          args.DB,
		running:               &atomic.Bool{},
		logger:                log.With(args.Logger, "collector", SetupActorsCollector),
		collectInterval:       args.CollectInterval,
		autoUpdateSetupActors: args.AutoUpdateSetupActors,
	}, nil
}

func (c *SetupActors) Name() string {
	return SetupActorsCollector
}

func (c *SetupActors) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "collector started")
	c.running.Store(true)

	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
		}()

		ticker := time.NewTicker(c.collectInterval)

		for {
			if err := c.checkSetupActors(c.ctx); err != nil {
				level.Error(c.logger).Log("msg", "collector error", "err", err)
			}

			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()

	return nil
}

func (c *SetupActors) Stopped() bool {
	return !c.running.Load()
}

func (c *SetupActors) Stop() {
	c.cancel()
	c.running.Store(false)
}

func (c *SetupActors) checkSetupActors(ctx context.Context) error {
	var user string
	err := c.dbConnection.QueryRowContext(ctx, selectUserQuery).Scan(&user)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to get current user", "err", err)
		return err
	}

	var enabled, history string
	err = c.dbConnection.QueryRowContext(ctx, selectQuery, user).Scan(&enabled, &history)
	if errors.Is(err, sql.ErrNoRows) {
		if c.autoUpdateSetupActors {
			return c.insertSetupActors(ctx, user)
		} else {
			level.Info(c.logger).Log("msg", "setup_actors configuration missing, but auto-update is disabled")
			return nil
		}
	} else if err != nil {
		level.Error(c.logger).Log("msg", "failed to query setup_actors table", "err", err)
		return err
	}

	if strings.ToUpper(enabled) != "NO" || strings.ToUpper(history) != "NO" {
		if c.autoUpdateSetupActors {
			return c.updateSetupActors(ctx, user, enabled, history)
		} else {
			level.Info(c.logger).Log("msg", "setup_actors configuration is not correct, but auto-update is disabled")
			return nil
		}
	}

	return nil
}

func (c *SetupActors) insertSetupActors(ctx context.Context, user string) error {
	_, err := c.dbConnection.ExecContext(ctx, insertQuery, user)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to insert setup_actors row", "err", err, "user", user)
		return err
	}

	level.Debug(c.logger).Log("msg", "inserted new setup_actors row", "user", user)
	return nil
}

func (c *SetupActors) updateSetupActors(ctx context.Context, user string, enabled string, history string) error {
	r, err := c.dbConnection.ExecContext(ctx, updateQuery, user)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to update setup_actors row", "err", err, "user", user)
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to get rows affected from setup_actors update", "err", err)
		return err
	}
	if rowsAffected == 0 {
		level.Error(c.logger).Log("msg", "no rows affected from setup_actors update", "user", user)
		return fmt.Errorf("no rows affected from setup_actors update")
	}

	level.Debug(c.logger).Log("msg", "updated setup_actors row", "rows_affected", rowsAffected, "previous_enabled", enabled, "previous_history", history, "user", user)
	return nil
}
