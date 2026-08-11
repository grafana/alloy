package collector

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/go-sqllexer"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	QueryDetailsCollector      = "query_details"
	OP_QUERY_ASSOCIATION       = "query_association"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
)

// selectQueryTextTemplate returns the query text for the tracked
// query_hashes on the currently connected database.
//
// TODO(cristian): consider fetching only the first query_sql_text
// and also using a cursor.
const selectQueryTextTemplate = `
	SELECT
		q.query_hash,
		qt.query_sql_text
	FROM
		sys.query_store_query q
	JOIN
		sys.query_store_query_text qt ON qt.query_text_id = q.query_text_id
	WHERE
		q.is_internal_query = 0
		AND q.query_hash IN (%s)`

// QueryTracker reports which (database, query_hash) pairs are currently tracked
// by the query_metrics collector.
type QueryTracker interface {
	GetQueryHashes(database string) []string
}

type QueryDetailsArguments struct {
	DB              *sql.DB
	CollectInterval time.Duration
	Tracker         QueryTracker
	EntryHandler    loki.EntryHandler

	Logger *slog.Logger
}

type QueryDetails struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	entryHandler    loki.EntryHandler
	tracker         QueryTracker
	obfuscator      *sqllexer.Obfuscator
	normalizer      *sqllexer.Normalizer

	// lastEmittedAt records the wall-clock time at which OP_QUERY_ASSOCIATION
	// was last emitted for a (database, query_hash), used to throttle logging
	// to at most one emission per EmitInterval per query.
	lastEmittedAt map[queryMetricsKey]time.Time

	// now allows overriding time.Now() in tests.
	now func() time.Time

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewQueryDetails(args QueryDetailsArguments) (*QueryDetails, error) {
	return &QueryDetails{
		dbConnection:    args.DB,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		tracker:         args.Tracker,
		obfuscator:      sqllexer.NewObfuscator(),
		normalizer: sqllexer.NewNormalizer(
			sqllexer.WithCollectTables(true),
			sqllexer.WithKeepIdentifierQuotation(true),
		),
		lastEmittedAt: map[queryMetricsKey]time.Time{},
		now:           time.Now,
		logger:        args.Logger.With("collector", QueryDetailsCollector),
		running:       &atomic.Bool{},
	}, nil
}

func (c *QueryDetails) Name() string {
	return QueryDetailsCollector
}

func (c *QueryDetails) Start(ctx context.Context) error {
	c.logger.Debug("collector started")

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			if err := c.collectWithTimeout(c.ctx); err != nil {
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

func (c *QueryDetails) Stopped() bool {
	return !c.running.Load()
}

func (c *QueryDetails) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *QueryDetails) collectWithTimeout(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.collect(ctx)
}

func (c *QueryDetails) collect(ctx context.Context) error {
	if c.tracker == nil {
		c.logger.Error("no query tracker available, skipping collection")
		return nil
	}

	database, ok := checkQueryStoreState(ctx, c.dbConnection, c.logger)
	if !ok {
		return nil
	}

	hashes := c.tracker.GetQueryHashes(database)
	if len(hashes) == 0 {
		// query_metrics is enabled but nothing is tracked yet: emit nothing.
		c.pruneThrottle(nil)
		return nil
	}

	query, args, err := buildQueryTextStatement(hashes)
	if err != nil {
		return err
	}

	rs, err := c.dbConnection.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to fetch query text: %w", err)
	}
	defer rs.Close()

	now := c.now()

	// seen collects every (database, query_hash) returned this cycle, including
	// throttled ones, so pruneThrottle only drops queries that truly left Query
	// Store rather than those merely skipped by the throttle.
	seen := map[queryMetricsKey]struct{}{}

	for rs.Next() {
		var hash []byte
		var queryText string
		if err := rs.Scan(&hash, &queryText); err != nil {
			c.logger.Error("failed to scan query text row", "err", err)
			continue
		}

		queryHash, err := formatQueryHash(hash)
		if err != nil {
			c.logger.Error("failed to format query hash", "err", err)
			continue
		}

		key := queryMetricsKey{database: database, queryHash: queryHash}
		if _, ok := seen[key]; ok {
			// A query_hash can map to multiple query_text_id rows; emit once.
			continue
		}
		seen[key] = struct{}{}

		if last, ok := c.lastEmittedAt[key]; ok && now.Sub(last) < database_observability.EmitInterval {
			continue
		}

		c.emit(database, queryHash, queryText)
		c.lastEmittedAt[key] = now
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over query text result set: %w", err)
	}

	c.pruneThrottle(seen)
	return nil
}

// emit obfuscates and normalizes the query text, then emits the
// query_association and query_parsed_table_name logs.
func (c *QueryDetails) emit(database, queryHash, queryText string) {
	normalized, metadata, err := sqllexer.ObfuscateAndNormalize(
		queryText, c.obfuscator, c.normalizer, sqllexer.WithDBMS(sqllexer.DBMSSQLServer),
	)
	if err != nil {
		c.logger.Warn("failed to normalize query text", "database", database, "query_hash", queryHash, "err", err)
		normalized = c.obfuscator.Obfuscate(queryText, sqllexer.WithDBMS(sqllexer.DBMSSQLServer))
		metadata = nil
	}

	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		OP_QUERY_ASSOCIATION,
		fmt.Sprintf(`database="%s" query_hash="%s" querytext=%q`, database, queryHash, normalized),
	)

	if metadata == nil {
		return
	}

	for _, table := range metadata.Tables {
		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_PARSED_TABLE_NAME,
			fmt.Sprintf(`database="%s" query_hash="%s" table="%s"`, database, queryHash, table),
		)
	}
}

// pruneThrottle drops throttle entries whose (database, query_hash) was not
// observed this cycle so the map stays bounded as queries leave Query Store.
func (c *QueryDetails) pruneThrottle(seen map[queryMetricsKey]struct{}) {
	for key := range c.lastEmittedAt {
		if _, ok := seen[key]; !ok {
			delete(c.lastEmittedAt, key)
		}
	}
}

// buildQueryTextStatement returns the query text statement plus its named
// parameters, binding each 16-char hex hash back to its binary(8) form.
func buildQueryTextStatement(hashes []string) (string, []any, error) {
	placeholders := make([]string, 0, len(hashes))
	args := make([]any, 0, len(hashes))
	for i, h := range hashes {
		raw, err := hex.DecodeString(h)
		if err != nil {
			return "", nil, fmt.Errorf("failed to decode tracked query hash %q: %w", h, err)
		}
		name := fmt.Sprintf("h%d", i)
		placeholders = append(placeholders, "@"+name)
		args = append(args, sql.Named(name, raw))
	}

	return fmt.Sprintf(selectQueryTextTemplate, strings.Join(placeholders, ", ")), args, nil
}
