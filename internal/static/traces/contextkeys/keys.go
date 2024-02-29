package contextkeys

type key int

const (
	// Logs is used to pass *logs.Logs through the context
	Logs key = iota

	// Metrics is used to pass instance.Manager through the context
	Metrics
)
