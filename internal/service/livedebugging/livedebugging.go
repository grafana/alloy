package livedebugging

import "sync"

type ComponentName string
type ComponentID string
type CallbackID string

// DebugCallbackManager is used to manage live debugging callbacks.
type DebugCallbackManager interface {
	DebugRegistry
	// AddCallback sets a callback for a given componentID.
	// The callback is used to send debugging data to live debugging consumers.
	AddCallback(callbackID CallbackID, componentID ComponentID, callback func(string))
	// DeleteCallback deletes a callback for a given componentID.
	DeleteCallback(callbackID CallbackID, componentID ComponentID)
}

// DebugDataPublisher is used by components to push information to live debugging consumers.
type DebugDataPublisher interface {
	DebugRegistry
	// Publish sends debugging data for a given componentID.
	Publish(componentID ComponentID, data string)
	// IsActive returns true when at least one consumer is listening for debugging data for the given componentID.
	IsActive(componentID ComponentID) bool
}

// DebugRegistry is used to keep track of the components that supports the live debugging functionality.
type DebugRegistry interface {
	// Register a component by name.
	Register(componentName ComponentName)
	// IsRegistered returns true if a component has live debugging support.
	IsRegistered(componentName ComponentName) bool
}

type liveDebugging struct {
	loadMut              sync.RWMutex
	callbacks            map[ComponentID]map[CallbackID]func(string)
	registeredComponents map[ComponentName]struct{}
}

var _ DebugCallbackManager = &liveDebugging{}
var _ DebugDataPublisher = &liveDebugging{}

// NewLiveDebugging creates a new instance of liveDebugging.
func NewLiveDebugging() *liveDebugging {
	return &liveDebugging{
		callbacks:            make(map[ComponentID]map[CallbackID]func(string)),
		registeredComponents: make(map[ComponentName]struct{}),
	}
}

func (s *liveDebugging) Publish(componentID ComponentID, data string) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()
	for _, callback := range s.callbacks[componentID] {
		callback(data)
	}
}

func (s *liveDebugging) IsActive(componentID ComponentID) bool {
	_, exist := s.callbacks[componentID]
	return exist
}

func (s *liveDebugging) AddCallback(callbackID CallbackID, componentID ComponentID, callback func(string)) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	if _, ok := s.callbacks[componentID]; !ok {
		s.callbacks[componentID] = make(map[CallbackID]func(string))
	}
	s.callbacks[componentID][callbackID] = callback
}

func (s *liveDebugging) DeleteCallback(callbackID CallbackID, componentID ComponentID) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	delete(s.callbacks[componentID], callbackID)
}

func (s *liveDebugging) Register(componentName ComponentName) {
	s.registeredComponents[componentName] = struct{}{}
}

func (s *liveDebugging) IsRegistered(componentName ComponentName) bool {
	_, exist := s.registeredComponents[componentName]
	return exist
}
