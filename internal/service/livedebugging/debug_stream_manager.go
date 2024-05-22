package livedebugging

import "sync"

// DebugStreamManager defines the operations for managing debug streams.
type DebugStreamManager interface {
	// Register a component by name
	Register(componentName string)
	// IsRegistered returns true if the component name was registered
	IsRegistered(componentName string) bool
	// Stream streams data for a given componentID.
	Stream(componentID string, data string)
	// SetStream assigns a debug stream callback to a componentID.
	SetStream(streamID string, componentID string, callback func(string))
	// DeleteStream removes a debug stream.
	DeleteStream(streamID string, componentID string)
}

type debugStreamManager struct {
	loadMut              sync.RWMutex
	streams              map[string]map[string]func(string)
	registeredComponents map[string]struct{}
}

// NewDebugStreamManager creates a new instance of DebugStreamManager.
func NewDebugStreamManager() *debugStreamManager {
	return &debugStreamManager{
		streams:              make(map[string]map[string]func(string)),
		registeredComponents: make(map[string]struct{}),
	}
}

var _ DebugStreamManager = &debugStreamManager{}

func (s *debugStreamManager) Stream(componentID string, data string) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()
	for _, stream := range s.streams[componentID] {
		stream(data)
	}
}

func (s *debugStreamManager) SetStream(streamID string, componentID string, callback func(string)) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	if _, ok := s.streams[componentID]; !ok {
		s.streams[componentID] = make(map[string]func(string))
	}
	s.streams[componentID][streamID] = callback
}

func (s *debugStreamManager) DeleteStream(streamID string, componentID string) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	delete(s.streams[componentID], streamID)
}

func (s *debugStreamManager) Register(componentName string) {
	s.registeredComponents[componentName] = struct{}{}
}

func (s *debugStreamManager) IsRegistered(componentName string) bool {
	_, exist := s.registeredComponents[componentName]
	return exist
}
