package syslog

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

var _ syslogtarget.DebugListener = (*liveDebuggingWriter)(nil)

type liveDebuggingWriter struct {
	componentID livedebugging.ComponentID
	pub         livedebugging.DebugDataPublisher
}

// newLiveDebuggingListener constructs a new live debugging adapter to listen for debug events.
//
// Returns stub implementation if live debugging service isn't available.
func newLiveDebuggingListener(opts component.Options) syslogtarget.DebugListener {
	// GetServiceData is a callback and may be nil in unit tests
	if opts.GetServiceData == nil {
		return syslogtarget.NopDebugListener{}
	}

	svc, err := opts.GetServiceData(livedebugging.ServiceName)
	if err != nil || svc == nil {
		return syslogtarget.NopDebugListener{}
	}

	publisher, ok := svc.(livedebugging.DebugDataPublisher)
	if !ok {
		return syslogtarget.NopDebugListener{}
	}

	return &liveDebuggingWriter{
		componentID: livedebugging.ComponentID(opts.ID),
		pub:         publisher,
	}
}

func (l liveDebuggingWriter) pushData(thunk func() string) {
	l.pub.PublishIfActive(livedebugging.Data{
		ComponentID: l.componentID,
		Type:        livedebugging.LokiLog,
		Count:       1,
		DataFunc:    thunk,
	})
}

// OnNewMessage implements syslogtarget.DebugListener.
func (l *liveDebuggingWriter) OnNewMessage(e syslogtarget.NewMessageDebugEvent) {
	l.pushData(func() string {
		sb := &strings.Builder{}
		sb.Grow(1 << 11) // 2 KiB
		fmt.Fprintf(
			sb, "[IN] New Log: Format=%q TS=%q\n  Message: %q\n",
			e.Format, e.Timestamp.Format(time.RFC3339), e.Message,
		)

		// print mapped and original labels to simplify relabel configuration debugging
		sb.WriteString("  Labels:\n")
		fmt.Fprintf(sb, "  - Mapped:   %s\n", e.MappedLabels)
		fmt.Fprintf(sb, "  - Original: %s\n", e.OriginalLabels)

		return sb.String()
	})
}
