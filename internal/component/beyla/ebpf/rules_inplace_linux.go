//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"fmt"
	"reflect"

	"golang.org/x/sys/unix"

	"github.com/grafana/alloy/internal/component/otelcol"
)

// applyRulesInPlace applies a change that touches nothing but dynamic_instrumentation by rewriting,
// in place, the in-memory config file the running Beyla was started with (`-config /proc/self/fd/N`).
// Beyla re-reads that file every dynamic_instrumentation.watch_interval and reconciles its rules:
// added rules attach, removed rules detach, and rules that did not change keep their patches and
// their remaining TTL. The process, its eBPF programs and its Java agent sessions stay as they are.
//
// It returns false, and writes nothing, whenever the update is anything else, so that the caller
// restarts Beyla as before. A failed write also returns false.
func (c *Component) applyRulesInPlace(newArgs Arguments) bool {
	if c.configFD <= 0 || c.subprocessCancel == nil {
		return false
	}

	// Beyla only watches its rules when it started with the rule engine on: the first rule of a Beyla
	// that started without one needs a restart.
	if !c.ruleEngineOn {
		return false
	}

	if !differOnlyInRules(c.args, newArgs) {
		return false
	}

	data, err := c.renderConfig(newArgs)

	if err != nil {
		c.opts.Logger.Warn("can't render the config for an in-place rule update, restarting Beyla", "err", err)
		return false
	}

	if err := rewriteMemfd(c.configFD, data); err != nil {
		c.opts.Logger.Warn("can't update the Beyla config in place, restarting Beyla", "err", err)
		return false
	}

	c.opts.Logger.Info("applied dynamic_instrumentation changes in place; Beyla was not restarted",
		"rules", len(newArgs.DynamicInstrumentation.Rules))

	return true
}

// differOnlyInRules reports whether a and b differ in dynamic_instrumentation and nowhere else.
func differOnlyInRules(a, b Arguments) bool {
	if !sameConsumers(a.Output, b.Output) {
		return false
	}

	a.Output, b.Output = nil, nil
	a.DynamicInstrumentation.Rules, b.DynamicInstrumentation.Rules = nil, nil // Enabled must still match

	return reflect.DeepEqual(a, b)
}

// sameConsumers compares the downstream consumers by identity: the OTLP receiver captured them at
// start, so a different consumer needs a restart.
func sameConsumers(a, b *otelcol.ConsumerArguments) (same bool) {
	if a == nil || b == nil {
		return a == b
	}

	defer func() {
		if recover() != nil { // an uncomparable consumer: treat as different
			same = false
		}
	}()

	eq := func(x, y []otelcol.Consumer) bool {
		if len(x) != len(y) {
			return false
		}

		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}

		return true
	}

	return eq(a.Metrics, b.Metrics) && eq(a.Logs, b.Logs) && eq(a.Traces, b.Traces)
}

// rewriteMemfd replaces the whole content of the memfd, keeping its inode (and so every path to it).
func rewriteMemfd(fd int, data []byte) error {
	for off := 0; off < len(data); {
		n, err := unix.Pwrite(fd, data[off:], int64(off))

		if err == unix.EINTR {
			continue
		}

		if err != nil {
			return fmt.Errorf("write: %w", err)
		}

		off += n
	}

	if err := unix.Ftruncate(fd, int64(len(data))); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	return nil
}
