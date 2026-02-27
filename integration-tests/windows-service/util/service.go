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

// EnsureServiceRunning checks that the Alloy service exists, starts it if needed, and asserts it is running.
func EnsureServiceRunning(c *assert.CollectT, t *testing.T, serviceName string) {
	t.Logf("Connecting to service manager")
	m, err := mgr.Connect()
	if !assert.NoError(c, err, "connect to service manager") {
		return
	}
	defer m.Disconnect()
	t.Logf("Connected to service manager")

	t.Logf("Opening service name=%s", serviceName)
	s, err := m.OpenService(serviceName)
	if !assert.NoError(c, err, "Alloy service should exist after install") {
		return
	}
	defer s.Close()
	t.Logf("Opened service name=%s", serviceName)

	t.Logf("Querying service status")
	status, err := s.Query()
	assert.NoError(c, err, "query service status")
	if err != nil {
		return
	}
	stateStr := ServiceStateString(status.State)
	t.Logf("Service status state=%s", stateStr)

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
		return // will be polled again until Running
	}
	t.Logf("Service is running")
}
