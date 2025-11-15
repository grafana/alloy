package source

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/runner"
)

// Source should be implemented by anything that can produce logs, like a file or http server.
type Source interface {
	runner.Worker
	runner.Task
}

// FIXME: add comments
type Host interface {
	// Reciever returns `loki.LogsReceiver` that all sources should send to.
	Reciever() loki.LogsReceiver
	// Positions returns `positions.Positions` that can be used to track what logs have been consumed.
	Positions() positions.Positions
	// Stopping returns true if Host is stopping.
	Stopping() bool
}

// SourceFactory is used by `Host` to create new sources. `Host` is responsible for scheduling and forward `loki.Entrie`
// it gets over recv to other components.
// FIXME: Some signal that Component is stopping should be passed
type SourceFactory interface {
	Sources(host Host, args component.Arguments) []Source
}
