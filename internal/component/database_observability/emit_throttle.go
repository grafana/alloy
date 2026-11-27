package database_observability

import "time"

// EmissionState records when a query's details were last emitted and whether its parsed tables need validation.
type EmissionState struct {
	// lastEmittedAt records the wall-clock time at which OP_QUERY_ASSOCIATION
	// was last emitted for a query details key, used to throttle logging
	// to at most one emission per EmitInterval per query.
	LastEmittedAt time.Time
	// PendingValidation marks queries whose parsed tables were emitted before schema
	//	// collection populated the database in tableRegistry, so they can be
	//	// re-emitted once when the database schema becomes available.
	PendingValidation bool
}

// EmissionStateByKey stores one emission record per query in a collector.
type EmissionStateByKey[K comparable] map[K]EmissionState

// PruneExpired removes records whose last emission was at least EmitInterval ago.
func (t EmissionStateByKey[K]) PruneExpired(now time.Time) {
	for key, last := range t {
		if now.Sub(last.LastEmittedAt) >= EmitInterval {
			delete(t, key)
		}
	}
}
