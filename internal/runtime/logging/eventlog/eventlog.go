package eventlog

// EventLog is the interface for writing to the Windows Event Log.
// It can be implemented by the real Windows event log or by a mock for testing.
type EventLog interface {
	Info(id uint32, msg string) error
	Warning(id uint32, msg string) error
	Error(id uint32, msg string) error
	Close() error
}

// EventLogOpener opens an event log for the given service name.
// When nil, the default opener is used (real on Windows, stub on other platforms).
type EventLogOpener func(serviceName string) (EventLog, error)
