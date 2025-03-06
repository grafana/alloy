package livedebugging

import (
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
)

type ComponentID string
type ModuleID string
type CallbackID string

// CallbackManager is used to manage live debugging callbacks.
type CallbackManager interface {
	// AddCallback sets a callback for a given componentID.
	// The callback is used to send debugging data to live debugging consumers.
	AddCallback(host service.Host, callbackID CallbackID, componentID ComponentID, callback func(Data)) error
	// DeleteCallback deletes a callback for a given componentID.
	DeleteCallback(host service.Host, callbackID CallbackID, componentID ComponentID)
	// AddCallbackMulti sets a callback to all components.
	// The callbacks are used to send debugging data to live debugging consumers.
	AddCallbackMulti(host service.Host, callbackID CallbackID, moduleID ModuleID, callback func(Data)) error
	// DeleteCallbackMulti deletes callbacks for all components.
	DeleteCallbackMulti(host service.Host, callbackID CallbackID, moduleID ModuleID)
}

// DebugDataPublisher is used by components to push information to live debugging consumers.
type DebugDataPublisher interface {
	// Publish sends debugging data for a given componentID if a least one consumer is listening for debugging data for the given componentID.
	PublishIfActive(data Data)
}
type liveDebugging struct {
	loadMut   sync.RWMutex
	callbacks map[ComponentID]map[CallbackID]func(Data)
	host      service.Host
	enabled   bool
}

var _ CallbackManager = &liveDebugging{}
var _ DebugDataPublisher = &liveDebugging{}

// NewLiveDebugging creates a new instance of liveDebugging.
func NewLiveDebugging() *liveDebugging {
	return &liveDebugging{
		callbacks: make(map[ComponentID]map[CallbackID]func(Data)),
	}
}

func (s *liveDebugging) PublishIfActive(data Data) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()

	if !s.enabled {
		return
	}

	if callbacks, exist := s.callbacks[data.ComponentID]; !exist || len(callbacks) == 0 {
		return
	}

	for _, callback := range s.callbacks[data.ComponentID] {
		callback(data)
	}
}

func (s *liveDebugging) AddCallback(host service.Host, callbackID CallbackID, componentID ComponentID, callback func(Data)) error {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()

	if !s.enabled {
		return fmt.Errorf("the live debugging service is disabled. Check the documentation to find out how to enable it")
	}

	if host == nil {
		return fmt.Errorf("the live debugging service is not ready yet")
	}

	info, err := host.GetComponent(component.ParseID(string(componentID)), component.InfoOptions{})
	if err != nil {
		return err
	}

	if _, ok := info.Component.(component.LiveDebugging); !ok {
		return fmt.Errorf("the component %q does not support live debugging", info.ComponentName)
	}

	if _, ok := s.callbacks[componentID]; !ok {
		s.callbacks[componentID] = make(map[CallbackID]func(Data))
	}

	s.callbacks[componentID][callbackID] = callback
	return nil
}

func (s *liveDebugging) AddCallbackMulti(host service.Host, callbackID CallbackID, moduleID ModuleID, callback func(Data)) error {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()

	// not needed here
	// if !s.enabled {
	// 	return fmt.Errorf("the live debugging service is disabled. Check the documentation to find out how to enable it")
	// }

	if host == nil {
		return fmt.Errorf("the live debugging service is not ready yet")
	}

	components, err := host.ListComponents(string(moduleID), component.InfoOptions{GetHealth: true})
	if err != nil {
		return err
	}

	for _, cp := range components {
		if _, ok := cp.Component.(component.LiveDebugging); !ok {
			continue // Ignore components that don't support live debugging
		}

		if _, ok := s.callbacks[ComponentID(cp.ID.String())]; !ok {
			s.callbacks[ComponentID(cp.ID.String())] = make(map[CallbackID]func(Data))
		}
		s.callbacks[ComponentID(cp.ID.String())][callbackID] = callback
	}
	return nil
}

func (s *liveDebugging) DeleteCallback(host service.Host, callbackID CallbackID, componentID ComponentID) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	delete(s.callbacks[componentID], callbackID)
}

func (s *liveDebugging) DeleteCallbackMulti(host service.Host, callbackID CallbackID, moduleID ModuleID) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	// ignore errors on delete
	components, _ := host.ListComponents(string(moduleID), component.InfoOptions{})
	for _, cp := range components {
		delete(s.callbacks[ComponentID(cp.ID.String())], callbackID)
	}
	// The s.callbacks[componentID] is not deleted. This is a very small memory leak which could only become significant if a user
	// has a lot of components and reload the config with always different component labels while having the graph open.
	// If this ever become a realistic scenario we should cleanup the map here.
}

// TODO: remove this function
func (s *liveDebugging) SetServiceHost(h service.Host) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	s.host = h
}

func (s *liveDebugging) SetEnabled(enabled bool) {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	s.enabled = enabled
}
