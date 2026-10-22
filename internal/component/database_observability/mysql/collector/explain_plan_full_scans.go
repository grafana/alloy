package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/database_observability"
)

// fullScanTTL is how long the full scans found in a query's explain plan are
// exposed without a newer plan. A query's plan is refreshed at most once per
// EmitInterval while the query keeps running, so a longer silence means the query
// stopped running, and its scans should stop being reported.
const fullScanTTL = 3 * database_observability.EmitInterval

var fullScanRowsDesc = prometheus.NewDesc(
	prometheus.BuildFQName("database_observability", "mysql_explain_plan", "full_scan_rows"),
	"Estimated rows the optimizer reads per scan of a table it reads in full (access type ALL), from the latest explain plan of the query. "+
		"has_possible_keys is \"false\" when MySQL found no index it could use for the query's conditions on that table. "+
		"The table label is the name the plan uses, which is the alias when the query gives one.",
	[]string{"schema", "digest", "table", "has_possible_keys"}, nil,
)

// fullScan is a table that an explain plan reads in full.
type fullScan struct {
	table string
	// rows is the optimizer's estimate of the rows read per scan of the table.
	rows float64
	// hasPossibleKeys is whether MySQL found any index it could use. When it is
	// false, no index fits the query's conditions on this table.
	hasPossibleKeys bool
}

// fullScansFromExplainJSON returns the tables that an `EXPLAIN FORMAT=JSON`
// plan reads in full, that is the tables with access type ALL. Tables that
// are not real ones, such as derived tables and union results, which MySQL
// names with a leading '<', are left out: no index could change them.
func fullScansFromExplainJSON(planJSON []byte) ([]fullScan, error) {
	decoder := json.NewDecoder(bytes.NewReader(planJSON))
	decoder.UseNumber()

	var plan any
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode explain plan: %w", err)
	}

	byTable := map[string]fullScan{}
	var order []string
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if scan, ok := fullScanOf(n); ok {
				// The same table can appear in more than one block, for example
				// in a subquery; keep the larger estimate and any usable index.
				if seen, dup := byTable[scan.table]; dup {
					scan.rows = max(scan.rows, seen.rows)
					scan.hasPossibleKeys = scan.hasPossibleKeys || seen.hasPossibleKeys
				} else {
					order = append(order, scan.table)
				}
				byTable[scan.table] = scan
			}
			for _, child := range n {
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(plan)

	scans := make([]fullScan, 0, len(order))
	for _, table := range order {
		scans = append(scans, byTable[table])
	}
	return scans, nil
}

// fullScanOf reads a plan node as a table access; ok is false when the node is
// not a table read in full.
func fullScanOf(node map[string]any) (fullScan, bool) {
	table, hasName := node["table_name"].(string)
	accessType, hasAccess := node["access_type"].(string)
	if !hasName || !hasAccess || !strings.EqualFold(accessType, "ALL") || strings.HasPrefix(table, "<") {
		return fullScan{}, false
	}

	scan := fullScan{table: table}
	if keys, ok := node["possible_keys"].([]any); ok && len(keys) > 0 {
		scan.hasPossibleKeys = true
	}
	if rows, ok := node["rows_examined_per_scan"].(json.Number); ok {
		if v, err := rows.Float64(); err == nil {
			scan.rows = v
		}
	}
	return scan, true
}

// fullScanStore keeps, for each query, the full scans of its latest explain
// plan.
type fullScanStore struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]fullScanEntry
}

type fullScanEntry struct {
	schema string
	digest string
	scans  []fullScan
	at     time.Time
}

func newFullScanStore(now func() time.Time) *fullScanStore {
	return &fullScanStore{now: now, entries: make(map[string]fullScanEntry)}
}

// set replaces the scans of a query. A plan without any full scan removes the
// query, so that a fixed query stops being reported as soon as it is re-planned.
func (s *fullScanStore) set(schema, digest string, scans []fullScan) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := explainPlanQueryKey(schema, digest)
	if len(scans) == 0 {
		delete(s.entries, key)
		return
	}
	s.entries[key] = fullScanEntry{schema: schema, digest: digest, scans: scans, at: s.now()}
}

// collect emits the scans that have not expired, and forgets the ones that have.
func (s *fullScanStore) collect(ch chan<- prometheus.Metric) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for key, e := range s.entries {
		if now.Sub(e.at) > fullScanTTL {
			delete(s.entries, key)
			continue
		}
		for _, scan := range e.scans {
			ch <- prometheus.MustNewConstMetric(fullScanRowsDesc, prometheus.GaugeValue, scan.rows,
				e.schema, e.digest, scan.table, fmt.Sprint(scan.hasPossibleKeys))
		}
	}
}
