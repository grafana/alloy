package database_observability

import "time"

const (
	JobName = "integrations/db-o11y"

	// EmitInterval is the minimum time between repeated emissions for the same
	// database object log lines, regardless of the configured collection interval.
	EmitInterval = 30 * time.Minute
)
