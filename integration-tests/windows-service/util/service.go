//go:build windows

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// ServiceExists returns true if the Alloy Windows service exists.
func ServiceExists(serviceName string) bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return false
	}
	_ = s.Close()
	return true
}

// ServiceStateString returns a human-readable name for svc.State for logging.
func ServiceStateString(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "StartPending"
	case svc.StopPending:
		return "StopPending"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "ContinuePending"
	case svc.PausePending:
		return "PausePending"
	case svc.Paused:
		return "Paused"
	default:
		return "Unknown"
	}
}

func queryService(c *assert.CollectT, t *testing.T, serviceName string) (m *mgr.Mgr, s *mgr.Service, status svc.Status, ok bool) {
	t.Logf("Connecting to service manager")
	m, err := mgr.Connect()
	if !assert.NoError(c, err, "connect to service manager") {
		return nil, nil, svc.Status{}, false
	}
	t.Logf("Connected to service manager")

	t.Logf("Opening service name=%s", serviceName)
	s, err = m.OpenService(serviceName)
	if !assert.NoError(c, err, "Alloy service should exist") {
		m.Disconnect()
		return nil, nil, svc.Status{}, false
	}
	t.Logf("Opened service name=%s", serviceName)

	t.Logf("Querying service status")
	status, err = s.Query()
	if !assert.NoError(c, err, "query service status") {
		s.Close()
		m.Disconnect()
		return nil, nil, svc.Status{}, false
	}
	t.Logf("Service status state=%s", ServiceStateString(status.State))

	return m, s, status, true
}

// EnsureServiceRunning checks that the Alloy service exists, starts it if needed, and asserts it is running.
func EnsureServiceRunning(c *assert.CollectT, t *testing.T, serviceName string) {
	m, s, status, ok := queryService(c, t, serviceName)
	if !ok {
		return
	}
	defer m.Disconnect()
	defer s.Close()

	if status.State != svc.Running {
		if status.State != svc.StartPending {
			t.Logf("Starting service (not running)")
			if err := s.Start(); err != nil {
				t.Logf("Start failed err=%v", err)
				assert.NoError(c, err, "start Alloy service")
				return
			}
			t.Logf("Start requested successfully")
		} else {
			t.Logf("Service is start pending, waiting")
		}
	}

	assert.Equal(c, svc.Running, status.State, "expected service to be running, got %s", ServiceStateString(status.State))
}

// EnsureServiceStopped checks that the Alloy service exists, stops it if
// needed, and asserts it is stopped
func EnsureServiceStopped(c *assert.CollectT, t *testing.T, serviceName string) {
	m, s, status, ok := queryService(c, t, serviceName)
	if !ok {
		return
	}
	defer m.Disconnect()
	defer s.Close()

	if status.State != svc.Stopped {
		if status.State != svc.StopPending {
			t.Logf("Stopping service (not stopped)")
			if _, err := s.Control(svc.Stop); err != nil {
				t.Logf("Stop failed err=%v", err)
				assert.NoError(c, err, "stop Alloy service")
				return
			}
			t.Logf("Stop requested successfully")
		} else {
			t.Logf("Service is stop pending, waiting")
		}
	}

	// Fail this tick until the service is actually stopped
	assert.Equal(c, svc.Stopped, status.State, "expected service to be stopped, got %s", ServiceStateString(status.State))
}
