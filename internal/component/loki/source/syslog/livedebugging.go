package syslog

import (
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
	svc, err := opts.GetServiceData(livedebugging.ServiceName)
	if err != nil {
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

// OnError implements syslogtarget.DebugListener.
func (l *liveDebuggingWriter) OnError(msg string, err error) {
	// TODO: implement
}

// OnNewMessage implements syslogtarget.DebugListener.
func (l *liveDebuggingWriter) OnNewMessage(e syslogtarget.NewMessageDebugEvent) {
	// TODO: implement
}
