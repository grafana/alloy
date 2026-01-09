package syslog

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"

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

// OnError implements syslogtarget.DebugListener.
func (l liveDebuggingWriter) OnError(msg string, err error) {
	if !l.pub.IsActive(l.componentID) {
		return
	}

	l.pushData(func() string {
		return fmt.Sprintf("[Error]: %s: %s", msg, err)
	})
}

// OnNewMessage implements syslogtarget.DebugListener.
func (l *liveDebuggingWriter) OnNewMessage(e syslogtarget.NewMessageDebugEvent) {
	if !l.pub.IsActive(l.componentID) {
		return
	}

	l.pushData(func() string {
		sb := &strings.Builder{}
		sb.Grow(1 << 11) // 2 KiB
		fmt.Fprintf(
			sb, "[IN] New Log: Format=%q TS=%q\n  Message: %q\n",
			e.Format, e.Timestamp.Format(time.RFC3339), e.Message,
		)

		// print mapped and original labels to simplify relabel configuration debugging
		sb.WriteString("  Mapped Labels:\n")
		if len(e.MappedLabels) == 0 {
			sb.WriteString("  <empty>\n")
		}

		for k, v := range e.MappedLabels {
			sb.WriteString("  - ")
			sb.WriteString(string(k))
			sb.WriteString(" = ")
			sb.WriteString(string(v))
			sb.WriteByte('\n')
		}

		sb.WriteString("  Original Labels:\n")
		if e.OriginalLabels.IsEmpty() {
			sb.WriteString("  <empty>\n")
		}

		e.OriginalLabels.Range(func(l labels.Label) {
			_, ok := e.MappedLabels[model.LabelName(l.Name)]
			if ok {
				return
			}

			sb.WriteString("  - ")
			sb.WriteString(l.Name)
			sb.WriteString(" = ")
			sb.WriteString(l.Value)
			sb.WriteByte('\n')
		})

		return sb.String()
	})
}
