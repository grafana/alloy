package testutil

import (
	"sync"
)

// MockEventLog is an EventLog implementation for testing on any OS.
type MockEventLog struct {
	mu       sync.Mutex
	Infos    []string
	Warnings []string
	Errors   []string
	closed   bool
}

func (m *MockEventLog) Info(id uint32, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Infos = append(m.Infos, msg)
	return nil
}
func (m *MockEventLog) Warning(id uint32, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Warnings = append(m.Warnings, msg)
	return nil
}
func (m *MockEventLog) Error(id uint32, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors = append(m.Errors, msg)
	return nil
}
func (m *MockEventLog) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// reset clears recorded messages for reuse in subtests.
func (m *MockEventLog) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Infos = nil
	m.Warnings = nil
	m.Errors = nil
}
