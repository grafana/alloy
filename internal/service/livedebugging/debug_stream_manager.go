package livedebugging

import "sync"

// DebugStreamManager defines the operations for managing debug streams.
type DebugStreamManager interface {
	// GetStream retrieves a debug stream callback by componentID.
	GetStream(id string) func(string)
	// SetStream assigns a debug stream callback to a componentID.
	SetStream(id string, callback func(string))
	// DeleteStream removes a debug stream by componentID.
	DeleteStream(id string)
}

type debugStreamManager struct {
	loadMut sync.RWMutex
	streams map[string]func(string)
}

// NewDebugStreamManager creates a new instance of DebugStreamManager.
func NewDebugStreamManager() *debugStreamManager {
	return &debugStreamManager{
		streams: make(map[string]func(string)),
	}
}

var _ DebugStreamManager = &debugStreamManager{}

func (s *debugStreamManager) GetStream(id string) func(string) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()
	return s.streams[id]
}

func (s *debugStreamManager) SetStream(id string, callback func(string)) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	s.streams[id] = callback
}

func (s *debugStreamManager) DeleteStream(id string) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	delete(s.streams, id)
}
