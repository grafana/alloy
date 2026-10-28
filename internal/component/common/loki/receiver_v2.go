package loki

import "fmt"

// NewLogsReceiverV2 returns a LogsReceiver for Loki pipeline V2 that passes
// every batch it receives to Consumer.
func NewLogsReceiverV2(c Consumer, componentID string) *LogsReceiverV2 {
	return &LogsReceiverV2{Consumer: c, componentID: componentID}
}

// LogsReceiverV2 is a LogsReceiver that receives batches through
// Consumer.Consume instead of a channel. It is exported by components when Loki pipeline V2 is enabled.
type LogsReceiverV2 struct {
	Consumer
	componentID string
}

// Chan panics. LogsReceiverV2 has no channel, so it can only be used by components that support Loki pipeline V2.
func (l *LogsReceiverV2) Chan() chan Entry {
	panic(fmt.Sprintf("fatal error in experimental Loki pipeline V2: %s only supports Loki pipeline V2", l.String()))
}

func (l *LogsReceiverV2) String() string {
	return l.componentID + ".receiver"
}
