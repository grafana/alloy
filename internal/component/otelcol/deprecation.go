package otelcol

import "log/slog"

// DeprecationLogger is implemented by component Arguments that want to log a
// warning for any deprecated setting currently in use. The receiver, exporter,
// and processor generic component wrappers check for it, via an optional
// interface assertion, in their shared Update method; since each wrapper's New
// constructor calls Update internally, a component picks this up on both its
// initial start and every subsequent configuration change just by implementing
// the method — no change needed to receiver.Arguments/exporter.Arguments/
// processor.Arguments. The connector, auth, and extension wrappers do not check
// for it yet.
//
// The runtime's own change-detection (skipping Update when the new Arguments
// equal the previous ones) means this doesn't re-log on every unchanged config
// reload — only when something in the component's configuration actually changes.
type DeprecationLogger interface {
	LogDeprecations(logger *slog.Logger)
}
