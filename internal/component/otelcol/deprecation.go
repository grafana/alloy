package otelcol

import "log/slog"

// DeprecationLogger is implemented by component Arguments that want to log a
// warning for any deprecated setting currently in use. The receiver, exporter,
// and processor wrappers check for it in their shared Update method, so it
// fires on both initial start and config changes with no other wiring needed.
// The connector, auth, and extension wrappers don't check for it yet.
type DeprecationLogger interface {
	LogDeprecations(logger *slog.Logger)
}
