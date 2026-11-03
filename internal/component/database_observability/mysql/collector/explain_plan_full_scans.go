package collector

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/database_observability"
)

const (
	// fullScanIdleTimeout is how long the full scans of a query are exposed
	// after the query was last seen running in the digest table. While it keeps
	// running, its scans stay exposed however long it waits for its next
	// explain plan; once it stops, they stop being reported.
	fullScanIdleTimeout = fullScanTimeBuckets * database_observability.EmitInterval
	// fullScanTimeBuckets is how many EmitInterval buckets hold the recent
	// running time of a query, which together cover fullScanIdleTimeout.
	fullScanTimeBuckets = 3

	// fullScanMinRows leaves out scans of tiny tables, which the optimizer
	// reads in full on purpose because that is cheaper than an index lookup.
	fullScanMinRows = 1000
	// fullScanMaxSeries caps the series exposed. The queries that spent the
	// most time running recently are kept.
	fullScanMaxSeries = 100
)

var fullScanRowsDesc = prometheus.NewDesc(
	prometheus.BuildFQName("database_observability", "mysql_explain_plan", "full_scan_rows"),
	"Estimated rows the optimizer reads per scan of a table it reads in full (access type ALL), from the latest explain plan of the query. "+
		"schema is the schema the query runs in; table_schema and table are the table's own schema and name. "+
		"When the table's real name can't be resolved, table is the name the plan uses, which is the alias when the query gives one, and table_schema is empty. "+
		"has_possible_index is \"false\" when MySQL found no index it could use for the query's conditions on that table.",
	[]string{"schema", "digest", "table_schema", "table", "has_possible_index"}, nil,
)

// fullScan is a table that an explain plan reads in full.
type fullScan struct {
	// tableSchema is empty when the table's real name could not be resolved,
	// in which case table is the name the plan uses.
	tableSchema string
	table       string
	// rows is the optimizer's estimate of the rows read per scan of the table.
	rows float64
	// hasPossibleIndex is whether MySQL found any index it could use. When it is
	// false, no index fits the query's conditions on this table.
	hasPossibleIndex bool
}

// fullScansFromExplainJSON returns the tables that an `EXPLAIN FORMAT=JSON`
// plan reads in full, that is the tables with access type ALL, with the real
// names that names resolves them to. Tables that are not real ones, such as
// derived tables, materialized CTEs and union results, which MySQL names with a
// leading '<', are left out: no index could change them.
func fullScansFromExplainJSON(planJSON []byte, names planTableNames) ([]fullScan, error) {
	decoder := json.NewDecoder(bytes.NewReader(planJSON))
	decoder.UseNumber()

	var plan any
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode explain plan: %w", err)
	}

	byTable := map[tableRef]fullScan{}
	var order []tableRef
	var walk func(node any, selectID int)
	walk = func(node any, selectID int) {
		switch n := node.(type) {
		case map[string]any:
			// A query block numbers its tables the way the rewritten query does.
			if id, ok := n["select_id"].(json.Number); ok {
				if v, err := id.Int64(); err == nil {
					selectID = int(v)
				}
			}
			if scan, ok := fullScanOf(n, selectID, names); ok {
				// The same table can appear in more than one block, for example
				// in a subquery; keep the larger estimate and any usable index.
				key := tableRef{schema: scan.tableSchema, table: scan.table}
				if seen, dup := byTable[key]; dup {
					scan.rows = max(scan.rows, seen.rows)
					scan.hasPossibleIndex = scan.hasPossibleIndex || seen.hasPossibleIndex
				} else {
					order = append(order, key)
				}
				byTable[key] = scan
			}
			for _, child := range n {
				walk(child, selectID)
			}
		case []any:
			for _, child := range n {
				walk(child, selectID)
			}
		}
	}
	walk(plan, 0)

	scans := make([]fullScan, 0, len(order))
	for _, key := range order {
		scans = append(scans, byTable[key])
	}
	return scans, nil
}

// fullScanOf reads a plan node as a table access; ok is false when the node is
// not a real table read in full.
func fullScanOf(node map[string]any, selectID int, names planTableNames) (fullScan, bool) {
	table, hasName := node["table_name"].(string)
	accessType, hasAccess := node["access_type"].(string)
	if !hasName || !hasAccess || !strings.EqualFold(accessType, "ALL") || strings.HasPrefix(table, "<") {
		return fullScan{}, false
	}

	scan := fullScan{table: table}
	switch ref, kind := names.resolve(selectID, table); kind {
	case planTableDerived:
		return fullScan{}, false
	case planTableReal:
		scan.tableSchema, scan.table = ref.schema, ref.table
	}
	if keys, ok := node["possible_keys"].([]any); ok && len(keys) > 0 {
		scan.hasPossibleIndex = true
	}
	if rows, ok := node["rows_examined_per_scan"].(json.Number); ok {
		if v, err := rows.Float64(); err == nil {
			scan.rows = v
		}
	}
	return scan, true
}

// fullScanStore keeps, for each query, the full scans of its latest explain
// plan and when the query was last seen running.
type fullScanStore struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]fullScanEntry
}

type fullScanEntry struct {
	schema string
	digest string
	scans  []fullScan
	// lastSeen is when the query was last explained or seen in the digest
	// table, whichever is later.
	lastSeen time.Time
	runTime  recentRunTime
}

// recentRunTime is the time a query spent running over the last
// fullScanIdleTimeout, from the growth of its SUM_TIMER_WAIT between polls of
// the digest table, kept in one bucket per EmitInterval.
type recentRunTime struct {
	// lastTimerWait is SUM_TIMER_WAIT at the previous poll; hasTimerWait is
	// false until the first poll, which only sets it.
	lastTimerWait uint64
	hasTimerWait  bool
	// buckets[0] starts at bucketStart, buckets[1] one EmitInterval before.
	buckets     [fullScanTimeBuckets]uint64
	bucketStart time.Time
}

func (r *recentRunTime) add(now time.Time, sumTimerWait uint64) {
	r.advance(now)
	if r.hasTimerWait {
		if sumTimerWait >= r.lastTimerWait {
			r.buckets[0] += sumTimerWait - r.lastTimerWait
		} else {
			// The digest table was truncated or the server restarted.
			r.buckets[0] += sumTimerWait
		}
	}
	r.lastTimerWait, r.hasTimerWait = sumTimerWait, true
}

// advance moves the buckets forward to now, dropping the ones that fell out of
// the window.
func (r *recentRunTime) advance(now time.Time) {
	if r.bucketStart.IsZero() {
		r.bucketStart = now
		return
	}
	for i := 0; i < fullScanTimeBuckets && now.Sub(r.bucketStart) >= database_observability.EmitInterval; i++ {
		copy(r.buckets[1:], r.buckets[:fullScanTimeBuckets-1])
		r.buckets[0] = 0
		r.bucketStart = r.bucketStart.Add(database_observability.EmitInterval)
	}
	if now.Sub(r.bucketStart) >= database_observability.EmitInterval {
		r.bucketStart = now
	}
}

func (r *recentRunTime) total(now time.Time) uint64 {
	r.advance(now)
	var total uint64
	for _, b := range r.buckets {
		total += b
	}
	return total
}

func newFullScanStore(now func() time.Time) *fullScanStore {
	return &fullScanStore{now: now, entries: make(map[string]fullScanEntry)}
}

// set replaces the scans of a query, leaving out the scans of tiny tables. A
// plan without any other full scan removes the query, so that a fixed query
// stops being reported as soon as it is re-planned.
func (s *fullScanStore) set(schema, digest string, scans []fullScan) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scans = slices.DeleteFunc(slices.Clone(scans), func(scan fullScan) bool { return scan.rows < fullScanMinRows })
	key := explainPlanQueryKey(schema, digest)
	if len(scans) == 0 {
		delete(s.entries, key)
		return
	}
	e := s.entries[key]
	e.schema, e.digest, e.scans, e.lastSeen = schema, digest, scans, s.now()
	s.entries[key] = e
}

// seen records that a query is still running, with its SUM_TIMER_WAIT, which
// keeps its scans exposed until its next explain plan replaces them.
func (s *fullScanStore) seen(schema, digest string, sumTimerWait uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := explainPlanQueryKey(schema, digest)
	if e, ok := s.entries[key]; ok {
		e.lastSeen = s.now()
		e.runTime.add(e.lastSeen, sumTimerWait)
		s.entries[key] = e
	}
}

// collect emits the scans of the queries seen running recently, and forgets
// the ones of queries that stopped. Up to fullScanMaxSeries series are emitted,
// from the queries that spent the most time running recently, with all the
// tables of a query or none; it returns how many series were left out.
func (s *fullScanStore) collect(ch chan<- prometheus.Metric) (dropped int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type rankedEntry struct {
		fullScanEntry
		runTime uint64
		rows    float64
	}
	now := s.now()
	ranked := make([]rankedEntry, 0, len(s.entries))
	for key, e := range s.entries {
		if now.Sub(e.lastSeen) > fullScanIdleTimeout {
			delete(s.entries, key)
			continue
		}
		r := rankedEntry{fullScanEntry: e, runTime: e.runTime.total(now)}
		for _, scan := range e.scans {
			r.rows = max(r.rows, scan.rows)
		}
		s.entries[key] = e
		ranked = append(ranked, r)
	}
	slices.SortFunc(ranked, func(a, b rankedEntry) int {
		return cmp.Or(
			cmp.Compare(b.runTime, a.runTime),
			cmp.Compare(b.rows, a.rows),
			cmp.Compare(a.schema, b.schema),
			cmp.Compare(a.digest, b.digest),
		)
	})

	emitted := 0
	for _, e := range ranked {
		if emitted+len(e.scans) > fullScanMaxSeries {
			dropped += len(e.scans)
			continue
		}
		emitted += len(e.scans)
		for _, scan := range e.scans {
			ch <- prometheus.MustNewConstMetric(fullScanRowsDesc, prometheus.GaugeValue, scan.rows,
				e.schema, e.digest, scan.tableSchema, scan.table, fmt.Sprint(scan.hasPossibleIndex))
		}
	}
	return dropped
}
