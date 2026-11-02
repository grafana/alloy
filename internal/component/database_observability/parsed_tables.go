package database_observability

// ParsedTable is a table extracted from query text and its schema validation status.
type ParsedTable struct {
	Table     string `json:"table"`
	Validated bool   `json:"validated"`
}
