package postgres

import (
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// receiverPump connects one exported logs receiver to the running dbInstance
// of that database. It is the exported receiver's only reader: entries are
// forwarded to the instance's logs collector while one is running, and
// discarded otherwise (database unreachable, or instance being replaced).
// This keeps the exported receiver drained at all times, so upstream
// components can never block on a send to it.
//
// The pump (and its exported receiver) lives for the whole component
// lifetime, surviving Updates that rebuild the instances, so downstream
// references to the exported receiver stay valid.
//
// Delivery around a target swap is best-effort: an entry received just
// before setTarget or clearTarget may be delivered to the newly installed
// target, or dropped. Successive targets always belong to the same
// database, so a swap can at most reorder, duplicate, or drop one
// in-flight entry.
type receiverPump struct {
	exported loki.LogsReceiver
	target   atomic.Pointer[pumpTarget]
}

type pumpTarget struct {
	receiver loki.LogsReceiver // the instance's internal logs receiver
	done     chan struct{}     // closed when the instance stops draining
}

func newReceiverPump() *receiverPump {
	return &receiverPump{exported: loki.NewLogsReceiver()}
}

// setTarget starts forwarding entries to the given receiver.
func (p *receiverPump) setTarget(receiver loki.LogsReceiver) {
	if old := p.target.Swap(&pumpTarget{receiver: receiver, done: make(chan struct{})}); old != nil {
		close(old.done)
	}
}

// clearTarget stops forwarding: subsequent entries are discarded, and an
// in-flight forward is abandoned so the pump can't block on a receiver that
// nothing drains anymore.
func (p *receiverPump) clearTarget() {
	if old := p.target.Swap(nil); old != nil {
		close(old.done)
	}
}

func (p *receiverPump) run(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case entry := <-p.exported.Chan():
			target := p.target.Load()
			if target == nil {
				continue
			}
			select {
			case target.receiver.Chan() <- entry:
			case <-target.done:
				// The instance stopped while forwarding; drop the entry.
			case <-stop:
				return
			}
		}
	}
}
