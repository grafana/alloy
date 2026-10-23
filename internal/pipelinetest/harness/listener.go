package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
)

// listenerReady reports whether a component of a given type is listening on
// an address, from its component info.
type listenerReady func(info *component.Info, address string) (bool, error)

// listenerChecks holds the component types that pipeline test inputs can wait
// for. A component type needs an entry here before a test can send it input
// that would be lost if sent too early, such as UDP datagrams.
var listenerChecks = map[string]listenerReady{
	// loki.source.gelf binds its listener while it is built, before the config
	// finishes loading, so it is ready as soon as it exists.
	"loki.source.gelf": func(*component.Info, string) (bool, error) { return true, nil },

	// loki.source.syslog binds its listeners after the config loads, and only
	// lists a listener in its debug info once it is bound.
	"loki.source.syslog": func(info *component.Info, address string) (bool, error) {
		var debugInfo struct {
			ListenersInfo []struct {
				ListenAddress string
			}
		}
		if err := decodeDebugInfo(info.DebugInfo, &debugInfo); err != nil {
			return false, err
		}
		for _, l := range debugInfo.ListenersInfo {
			if l.ListenAddress == address {
				return true, nil
			}
		}
		return false, nil
	},
}

// ListenerComponentTypes returns the component types WaitForListener supports.
func ListenerComponentTypes() []string {
	types := make([]string, 0, len(listenerChecks))
	for t := range listenerChecks {
		types = append(types, t)
	}
	slices.Sort(types)
	return types
}

// SupportsListener reports whether WaitForListener supports the component with
// the given ID.
func SupportsListener(id string) bool {
	_, ok := listenerChecks[componentType(id)]
	return ok
}

// WaitForListener waits until the component with the given ID listens on
// address, or the timeout passes.
func (a *Alloy) WaitForListener(ctx context.Context, id, address string, timeout time.Duration) error {
	check, ok := listenerChecks[componentType(id)]
	if !ok {
		return fmt.Errorf("waiting for %s is not supported; supported component types: %s", id, strings.Join(ListenerComponentTypes(), ", "))
	}

	deadline := time.Now().Add(timeout)
	for {
		info, err := a.ctrl.GetComponent(component.ParseID(id), component.InfoOptions{GetDebugInfo: true})
		if err != nil {
			return fmt.Errorf("get component %s: %w", id, err)
		}
		ready, err := check(info, address)
		if err != nil {
			return fmt.Errorf("check listener of %s: %w", id, err)
		}
		if ready {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%s is not listening on %s after %s", id, address, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(defaultInterval):
		}
	}
}

// componentType returns the type of a component from its ID, such as
// loki.source.syslog for loki.source.syslog.default. Labels cannot contain
// dots, so the type is everything before the last one.
func componentType(id string) string {
	i := strings.LastIndex(id, ".")
	if i < 0 {
		return id
	}
	return id[:i]
}

// decodeDebugInfo copies a component's debug info into out. Debug info types
// are usually unexported in their component's package, so it goes through
// JSON, which keeps exported field names.
func decodeDebugInfo(debugInfo any, out any) error {
	raw, err := json.Marshal(debugInfo)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
