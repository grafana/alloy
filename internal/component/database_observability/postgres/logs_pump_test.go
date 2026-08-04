package postgres

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func pumpTestEntry(line string) loki.Entry {
	return loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: line}}
}

// sendToPump sends an entry to the pump's exported receiver, failing the test
// if the send blocks: the pump must always drain the exported receiver.
func sendToPump(t *testing.T, pump *receiverPump, line string) {
	t.Helper()
	select {
	case pump.exported.Chan() <- pumpTestEntry(line):
	case <-time.After(5 * time.Second):
		t.Fatal("send to exported receiver blocked; pump is not draining")
	}
}

func Test_receiverPump(t *testing.T) {
	startPump := func(t *testing.T) *receiverPump {
		t.Helper()
		pump := newReceiverPump()
		stop := make(chan struct{})
		t.Cleanup(func() { close(stop) })
		go pump.run(stop)
		return pump
	}

	// expectForwarded waits for the given line to arrive on the target
	// receiver. Delivery around a target swap is best-effort: an entry the
	// pump received just before the swap may be delivered to the newly
	// installed target instead of being dropped, so earlier lines are
	// skipped rather than asserted against.
	expectForwarded := func(t *testing.T, target loki.LogsReceiver, line string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case got := <-target.Chan():
				if got.Line == line {
					return
				}
			case <-deadline:
				t.Fatalf("entry %q was not forwarded to the target receiver", line)
			}
		}
	}

	t.Run("discards entries when no instance is running", func(t *testing.T) {
		pump := startPump(t)

		for range 3 {
			sendToPump(t, pump, "discarded")
		}
	})

	t.Run("forwards entries to the target receiver", func(t *testing.T) {
		pump := startPump(t)

		target := loki.NewLogsReceiver()
		pump.setTarget(target)

		go func() { pump.exported.Chan() <- pumpTestEntry("forwarded") }()
		expectForwarded(t, target, "forwarded")
	})

	t.Run("clearTarget abandons an in-flight forward without blocking", func(t *testing.T) {
		pump := startPump(t)

		// A target nobody drains: the pump blocks forwarding the first entry.
		pump.setTarget(loki.NewLogsReceiver())
		sendToPump(t, pump, "stuck")

		// Clearing the target drops the in-flight entry, and subsequent
		// entries are discarded rather than blocking the sender.
		pump.clearTarget()
		sendToPump(t, pump, "discarded")

		// A new target receives entries again.
		target := loki.NewLogsReceiver()
		pump.setTarget(target)
		go func() { pump.exported.Chan() <- pumpTestEntry("forwarded") }()
		expectForwarded(t, target, "forwarded")
	})

	t.Run("setTarget replaces an undrained target without blocking", func(t *testing.T) {
		pump := startPump(t)

		pump.setTarget(loki.NewLogsReceiver())
		sendToPump(t, pump, "stuck")

		target := loki.NewLogsReceiver()
		pump.setTarget(target)
		go func() { pump.exported.Chan() <- pumpTestEntry("forwarded") }()
		expectForwarded(t, target, "forwarded")
	})
}
