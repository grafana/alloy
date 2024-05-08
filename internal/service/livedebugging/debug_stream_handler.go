package livedebugging

// DebugStreamHandler defines the operations for managing debug streams.
type DebugStreamHandler interface {
	// GetStream retrieves a debug stream callback by componentID.
	GetStream(id string) func(string)
	// SetStream assigns a debug stream callback to a componentID.
	SetStream(id string, callback func(string))
	// DeleteStream removes a debug stream by componentID.
	DeleteStream(id string)
}
