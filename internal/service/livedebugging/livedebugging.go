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
	AddCallback(callbackID CallbackID, componentID ComponentID, callback func(FeedData)) error
	// DeleteCallback deletes a callback for a given componentID.
	DeleteCallback(callbackID CallbackID, componentID ComponentID)

	AddMultiCallback(callbackID CallbackID, moduleID ModuleID, callback func(FeedData)) error
	DeleteMultiCallback(callbackID CallbackID, moduleID ModuleID)
}

// DebugDataPublisher is used by components to push information to live debugging consumers.
type DebugDataPublisher interface {
	// Publish sends debugging data for a given componentID.
	Publish(componentID ComponentID, data FeedData)
	// IsActive returns true when at least one consumer is listening for debugging data for the given componentID.
	IsActive(componentID ComponentID) bool
}

type FeedDataType string

const (
	Target           FeedDataType = "target"
	PrometheusMetric FeedDataType = "prometheus_metric"
	LokiLog          FeedDataType = "loki_log"
	OtelMetric       FeedDataType = "otel_metric"
	OtelLog          FeedDataType = "otel_log"
	OtelTrace        FeedDataType = "otel_trace"
)

type FeedData struct {
	ComponentID ComponentID  `json:"componentID"`
	Type        FeedDataType `json:"type"`
	Count       int          `json:"count"`
	Data        string       `json:"data"`
}

type liveDebugging struct {
	loadMut   sync.RWMutex
	callbacks map[ComponentID]map[CallbackID]func(FeedData)
	host      service.Host
	enabled   bool
}

var _ CallbackManager = &liveDebugging{}
var _ DebugDataPublisher = &liveDebugging{}

// NewLiveDebugging creates a new instance of liveDebugging.
func NewLiveDebugging() *liveDebugging {
	return &liveDebugging{
		callbacks: make(map[ComponentID]map[CallbackID]func(FeedData)),
	}
}

func (s *liveDebugging) Publish(componentID ComponentID, data FeedData) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()
	if s.enabled {
		for _, callback := range s.callbacks[componentID] {
			callback(data)
		}
	}
}

func (s *liveDebugging) IsActive(componentID ComponentID) bool {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()
	callbacks, exist := s.callbacks[componentID]
	return exist && len(callbacks) > 0
}

func (s *liveDebugging) AddCallback(callbackID CallbackID, componentID ComponentID, callback func(FeedData)) error {
	err := s.addCallback(callbackID, componentID, callback)
	if err != nil {
		return err
	}
	s.notifyComponent(componentID)
	return nil
}

func (s *liveDebugging) AddMultiCallback(callbackID CallbackID, moduleID ModuleID, callback func(FeedData)) error {
	err := s.addMultiCallback(callbackID, moduleID, callback)
	if err != nil {
		return err
	}
	s.notifyComponents(moduleID)
	return nil
}

func (s *liveDebugging) DeleteCallback(callbackID CallbackID, componentID ComponentID) {
	defer s.notifyComponent(componentID)
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	delete(s.callbacks[componentID], callbackID)
}

func (s *liveDebugging) DeleteMultiCallback(callbackID CallbackID, moduleID ModuleID) {
	defer s.notifyComponents(moduleID)
	s.loadMut.Lock()
	defer s.loadMut.Unlock()
	// ignore errors on delete
	components, _ := s.host.ListComponents(string(moduleID), component.InfoOptions{})
	for _, cp := range components {
		delete(s.callbacks[ComponentID(cp.ID.String())], callbackID)
	}
}

func (s *liveDebugging) addCallback(callbackID CallbackID, componentID ComponentID, callback func(FeedData)) error {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()

	if !s.enabled {
		return fmt.Errorf("the live debugging service is disabled. Check the documentation to find out how to enable it")
	}

	if s.host == nil {
		return fmt.Errorf("the live debugging service is not ready yet")
	}

	info, err := s.host.GetComponent(component.ParseID(string(componentID)), component.InfoOptions{})
	if err != nil {
		return err
	}

	if _, ok := info.Component.(component.LiveDebugging); !ok {
		return fmt.Errorf("the component %q does not support live debugging", info.ComponentName)
	}

	if _, ok := s.callbacks[componentID]; !ok {
		s.callbacks[componentID] = make(map[CallbackID]func(FeedData))
	}
	s.callbacks[componentID][callbackID] = callback
	return nil
}

func (s *liveDebugging) addMultiCallback(callbackID CallbackID, moduleID ModuleID, callback func(FeedData)) error {
	s.loadMut.Lock()
	defer s.loadMut.Unlock()

	if !s.enabled {
		return fmt.Errorf("the live debugging service is disabled. Check the documentation to find out how to enable it")
	}

	if s.host == nil {
		return fmt.Errorf("the live debugging service is not ready yet")
	}

	components, err := s.host.ListComponents(string(moduleID), component.InfoOptions{GetHealth: true})
	if err != nil {
		return err
	}

	for _, cp := range components {
		if _, ok := cp.Component.(component.LiveDebugging); !ok {
			continue // Ignore components that dont support live debugging
		}

		if _, ok := s.callbacks[ComponentID(cp.ID.String())]; !ok {
			s.callbacks[ComponentID(cp.ID.String())] = make(map[CallbackID]func(FeedData))
		}
		s.callbacks[ComponentID(cp.ID.String())][callbackID] = callback
	}
	return nil
}

func (s *liveDebugging) notifyComponent(componentID ComponentID) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()

	info, err := s.host.GetComponent(component.ParseID(string(componentID)), component.InfoOptions{})
	if err != nil {
		return
	}
	if component, ok := info.Component.(component.LiveDebugging); ok {
		// notify the component of the change
		component.LiveDebugging(len(s.callbacks[componentID]))
	}
}

func (s *liveDebugging) notifyComponents(moduleID ModuleID) {
	s.loadMut.RLock()
	defer s.loadMut.RUnlock()

	components, err := s.host.ListComponents(string(moduleID), component.InfoOptions{})
	if err != nil {
		return
	}
	for _, cp := range components {
		if c, ok := cp.Component.(component.LiveDebugging); ok {
			// notify the component of the change
			c.LiveDebugging(len(s.callbacks[ComponentID(cp.ID.String())]))
		}
	}
}

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
