package collector

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	ExplainPlansCollector  = "explain_plans"
	OP_EXPLAIN_PLAN_OUTPUT = "explain_plan_output"
)

// selectExplainPlansTemplate resolves each tracked query_hash to the most
// recently active plan Query Store has already captured for it (no live
// SET SHOWPLAN_XML ON compilation - the plan is read from the same Query
// Store data query_metrics already joins). A query_hash can map to more than
// one query_id/plan_id (recompiles, parameter sniffing), so ROW_NUMBER picks
// exactly one plan per hash rather than enumerating every match.
const selectExplainPlansTemplate = `
	WITH ranked_plans AS (
		SELECT
			q.query_hash,
			p.query_plan,
			ROW_NUMBER() OVER (
				PARTITION BY q.query_hash
				ORDER BY MAX(i.end_time) DESC
			) AS rn
		FROM sys.query_store_query q
		JOIN sys.query_store_plan p ON p.query_id = q.query_id
		JOIN sys.query_store_runtime_stats rs ON rs.plan_id = p.plan_id
		JOIN sys.query_store_runtime_stats_interval i
			ON i.runtime_stats_interval_id = rs.runtime_stats_interval_id
		WHERE q.is_internal_query = 0
			AND q.query_hash IN (%s)
		GROUP BY q.query_hash, p.plan_id, p.query_plan
	)
	SELECT query_hash, query_plan
	FROM ranked_plans
	WHERE rn = 1`

type ExplainPlansArguments struct {
	DB              *sql.DB
	CollectInterval time.Duration
	// Tracker is query_metrics's tracked query_hash set. This is nil when
	// query_metrics is disabled, in which case explain_plans has no bounded
	// candidate set to work from and no-ops every cycle, mirroring
	// query_details's existing dependency on the same tracker.
	Tracker      QueryTracker
	EntryHandler loki.EntryHandler

	Logger *slog.Logger
}

type ExplainPlans struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	tracker         QueryTracker
	entryHandler    loki.EntryHandler

	// lastEmittedAt/lastEmittedHash gate emission: a plan is only logged again
	// once its structural hash changes, or once EmitInterval has elapsed since
	// the last emission - whichever comes first. This bounds volume without
	// ever going longer than EmitInterval without a fresh line for the UI to
	// pick up.
	lastEmittedAt   map[queryMetricsKey]time.Time
	lastEmittedHash map[queryMetricsKey]string

	// now allows overriding time.Now() in tests.
	now func() time.Time

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewExplainPlans(args ExplainPlansArguments) (*ExplainPlans, error) {
	return &ExplainPlans{
		dbConnection:    args.DB,
		collectInterval: args.CollectInterval,
		tracker:         args.Tracker,
		entryHandler:    args.EntryHandler,
		lastEmittedAt:   map[queryMetricsKey]time.Time{},
		lastEmittedHash: map[queryMetricsKey]string{},
		now:             time.Now,
		logger:          args.Logger.With("collector", ExplainPlansCollector),
		running:         atomic.NewBool(false),
	}, nil
}

func (c *ExplainPlans) Name() string {
	return ExplainPlansCollector
}

func (c *ExplainPlans) Start(ctx context.Context) error {
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

func (c *ExplainPlans) Stopped() bool {
	return !c.running.Load()
}

func (c *ExplainPlans) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *ExplainPlans) collectWithTimeout(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.collect(ctx)
}

func (c *ExplainPlans) collect(ctx context.Context) error {
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
		c.pruneStale(nil)
		return nil
	}

	query, args, err := buildExplainPlansStatement(hashes)
	if err != nil {
		return err
	}

	rs, err := c.dbConnection.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to fetch explain plans: %w", err)
	}
	defer rs.Close()

	now := c.now()
	seen := map[queryMetricsKey]struct{}{}
	found := map[string]struct{}{}

	for rs.Next() {
		var hashBytes []byte
		var planXML []byte
		if err := rs.Scan(&hashBytes, &planXML); err != nil {
			c.logger.Error("failed to scan explain plan row", "err", err)
			continue
		}

		queryHash, err := formatQueryHash(hashBytes)
		if err != nil {
			c.logger.Error("failed to format query hash", "err", err)
			continue
		}

		found[queryHash] = struct{}{}
		key := queryMetricsKey{database: database, queryHash: queryHash}
		seen[key] = struct{}{}

		c.processPlan(now, key, queryHash, database, planXML)
	}
	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over explain plan rows: %w", err)
	}

	// Tracked hashes with no captured plan yet still get a gated skip
	// emission, so the UI's "no explain plan available" reflects Query Store
	// not having a plan yet rather than looking like a silent gap.
	for _, hash := range hashes {
		if _, ok := found[hash]; ok {
			continue
		}
		key := queryMetricsKey{database: database, queryHash: hash}
		seen[key] = struct{}{}
		c.maybeEmitSkip(now, key, hash, database, "no captured plan for this query in Query Store")
	}

	c.pruneStale(seen)
	return nil
}

func (c *ExplainPlans) processPlan(now time.Time, key queryMetricsKey, queryHash, database string, planXML []byte) {
	planNode, err := newExplainPlanOutputFromShowPlanXML(planXML)
	if err != nil {
		c.logger.Error("failed to parse showplan xml", "query_hash", queryHash, "err", err)
		c.maybeEmitSkip(now, key, queryHash, database, fmt.Sprintf("failed to parse showplan xml: %s", err.Error()))
		return
	}

	hash, err := structuralHash(*planNode)
	if err != nil {
		// Skip this cycle entirely rather than recording a sentinel hash: a
		// bogus value here would either wrongly look "unchanged" against the
		// last real hash (suppressing a genuine change) or, if this recurs
		// intermittently, keep looking "changed" against it (defeating the
		// gate the same way grafana-dbo11y-app#3409 did). Leaving state
		// untouched means the next successful cycle still compares against
		// the last known-good hash.
		c.logger.Error("failed to compute structural hash for explain plan, skipping this cycle", "query_hash", queryHash, "err", err)
		return
	}
	if !c.shouldEmit(key, hash, now) {
		return
	}

	c.emit(database, queryHash, now, database_observability.ExplainProcessingResultSuccess, "", planNode)
	c.recordEmission(key, hash, now)
}

func (c *ExplainPlans) maybeEmitSkip(now time.Time, key queryMetricsKey, queryHash, database, reason string) {
	skipHash := "skip:" + reason
	if !c.shouldEmit(key, skipHash, now) {
		return
	}
	c.emit(database, queryHash, now, database_observability.ExplainProcessingResultSkipped, reason, nil)
	c.recordEmission(key, skipHash, now)
}

// shouldEmit gates emission on either the plan having changed since the last
// emission, or EmitInterval having elapsed - whichever comes first. This
// intentionally checks for a changed plan every collect_interval (poll cost
// is a cheap system-view read, not a live compile) while never emitting more
// often than that unless the plan actually changed, avoiding the unbounded
// per-poll re-emission that caused grafana-dbo11y-app#3409 for mysql/postgres.
func (c *ExplainPlans) shouldEmit(key queryMetricsKey, hash string, now time.Time) bool {
	prevHash, hasPrevHash := c.lastEmittedHash[key]
	prevTime, hasPrevTime := c.lastEmittedAt[key]
	if !hasPrevHash || !hasPrevTime {
		return true
	}
	if hash != prevHash {
		return true
	}
	return now.Sub(prevTime) >= database_observability.EmitInterval
}

func (c *ExplainPlans) recordEmission(key queryMetricsKey, hash string, now time.Time) {
	if c.lastEmittedAt == nil {
		c.lastEmittedAt = map[queryMetricsKey]time.Time{}
	}
	if c.lastEmittedHash == nil {
		c.lastEmittedHash = map[queryMetricsKey]string{}
	}
	c.lastEmittedAt[key] = now
	c.lastEmittedHash[key] = hash
}

// pruneStale drops gate state for keys that were not observed this cycle,
// i.e. hashes query_metrics is no longer tracking.
func (c *ExplainPlans) pruneStale(seen map[queryMetricsKey]struct{}) {
	for key := range c.lastEmittedAt {
		if _, ok := seen[key]; !ok {
			delete(c.lastEmittedAt, key)
			delete(c.lastEmittedHash, key)
		}
	}
}

func (c *ExplainPlans) emit(
	database, queryHash string,
	now time.Time,
	result database_observability.ExplainProcessingResult,
	reason string,
	plan *database_observability.ExplainPlanNode,
) {

	output := &database_observability.ExplainPlanOutput{
		Metadata: database_observability.ExplainPlanMetadataInfo{
			// DatabaseEngine/DatabaseVersion are intentionally left unset: the
			// engine is now carried as a Loki label (see addLokiLabels) instead,
			// which is queryable and consistent with connection_info's "engine"
			// metric label, unlike these fields which no UI consumer ever read
			// (grafana-dbo11y-app#3471). mysql/postgres still populate them
			// pending a follow-up change there.
			QueryIdentifier:        queryHash,
			GeneratedAt:            now.Format(time.RFC3339),
			ProcessingResult:       result,
			ProcessingResultReason: reason,
		},
	}
	if plan != nil {
		output.Plan = *plan
	}

	explainPlanOutputJSON, err := json.Marshal(output)
	if err != nil {
		c.logger.Error("failed to marshal explain plan output", "err", err)
		return
	}

	logMessage := fmt.Sprintf(
		`database="%s" query_hash="%s" explain_plan_output="%s"`,
		database,
		queryHash,
		base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
	)

	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		OP_EXPLAIN_PLAN_OUTPUT,
		logMessage,
	)
}

// structuralHash hashes the plan tree with EstimatedRows/EstimatedCost
// excluded, since those optimizer estimates jitter between polls even when
// the plan shape is unchanged (see grafana-dbo11y-app#3409). The error is the
// caller's signal to skip this cycle for this query rather than record a
// hash that isn't a genuine reflection of the plan's content.
func structuralHash(node database_observability.ExplainPlanNode) (string, error) {
	b, err := json.Marshal(stripVolatileFields(node))
	if err != nil {
		return "", fmt.Errorf("failed to marshal plan for hashing: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func stripVolatileFields(node database_observability.ExplainPlanNode) database_observability.ExplainPlanNode {
	node.Details.EstimatedRows = 0
	node.Details.EstimatedCost = nil
	if len(node.Children) > 0 {
		children := make([]database_observability.ExplainPlanNode, len(node.Children))
		for i, child := range node.Children {
			children[i] = stripVolatileFields(child)
		}
		node.Children = children
	}
	return node
}

// buildExplainPlansStatement returns the hash-resolution statement plus its
// named parameters, binding each 16-char hex hash back to its binary(8) form.
func buildExplainPlansStatement(hashes []string) (string, []any, error) {
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

	return fmt.Sprintf(selectExplainPlansTemplate, strings.Join(placeholders, ", ")), args, nil
}
