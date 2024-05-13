package livedebugging

import "sync"

// DebugStreamManager manages a set of debugging streams identified by componentID.
type DebugStreamManager struct {
	loadMut sync.RWMutex
	streams map[string]func(string)
}

// NewDebugStreamManager creates a new instance of DebugStreamManager.
func NewDebugStreamManager() *DebugStreamManager {
	return &DebugStreamManager{
		streams: make(map[string]func(string)),
	}
}

var _ DebugStreamHandler = &DebugStreamManager{}

func (s *DebugStreamManager) GetStream(id string) func(string) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()
	return s.streams[id]
}

func (s *DebugStreamManager) SetStream(id string, callback func(string)) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	s.streams[id] = callback
}

func (s *DebugStreamManager) DeleteStream(id string) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	delete(s.streams, id)
}
