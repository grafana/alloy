// Package simulation tests the lifecycle contract of log-forwarding
// components (see component.Component.Update) on the real Alloy runtime.
//
// The tests register fixture components (lokisim.source, lokisim.relay and
// lokisim.sink), load and reload configs through the runtime, and check that
// every entry a source produces reaches a sink exactly once, that reloads
// never deadlock, and that failures are reported and retried. The fixtures can
// inject faults: failing starts, exclusive resources such as ports, producers
// that block until their entries are handed off, slow and late downstreams.
//
// The source fixture also implements today's ad-hoc lifecycles ("legacy"
// modes), so the same scenarios show which of them deadlock.
package simulation
