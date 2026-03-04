package database_observability

import (
	"context"
	"database/sql"
)

// ConnectionInfoPingThreshold is the number of consecutive ping failures before
// the connection_info metric is unregistered, and the number of consecutive
// ping successes before it is re-registered.
const ConnectionInfoPingThreshold = 3

// ConnectionInfoToggler is implemented by the ConnectionInfo collector in each
// database engine package. It allows the component to toggle metric registration
// without importing a concrete collector type.
type ConnectionInfoToggler interface {
	IsRegistered() bool
	Unregister()
	Reregister()
}

// CIPingState tracks consecutive ping results for the connection_info metric
// toggle. It is intended to be goroutine-local (owned by Run()'s ticker loop)
// and requires no external locking.
type CIPingState struct {
	failures  int
	successes int
	lastCI    ConnectionInfoToggler
}

// PingConnectionInfo pings db and toggles the connection_info metric via
// toggler based on consecutive failure or success counts in state. It should
// be called once per ticker tick from the component's Run() loop.
//
// After ConnectionInfoPingThreshold consecutive failures, toggler.Unregister()
// is called. After ConnectionInfoPingThreshold consecutive successes (while
// unregistered), toggler.Reregister() is called. When toggler changes (i.e.
// the component reconnected and created a new collector), state resets.
func PingConnectionInfo(ctx context.Context, db *sql.DB, toggler ConnectionInfoToggler, state *CIPingState) {
	if toggler != state.lastCI {
		state.failures = 0
		state.successes = 0
		state.lastCI = toggler
	}

	if db == nil || toggler == nil {
		return
	}

	if err := db.PingContext(ctx); err != nil {
		state.successes = 0
		if toggler.IsRegistered() {
			state.failures++
			if state.failures >= ConnectionInfoPingThreshold {
				toggler.Unregister()
				state.failures = 0
			}
		}
	} else {
		state.failures = 0
		if !toggler.IsRegistered() {
			state.successes++
			if state.successes >= ConnectionInfoPingThreshold {
				toggler.Reregister()
				state.successes = 0
			}
		}
	}
}
