package database_observability

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type mockToggler struct {
	registered bool
}

func (m *mockToggler) IsRegistered() bool { return m.registered }
func (m *mockToggler) Unregister()        { m.registered = false }
func (m *mockToggler) Reregister()        { m.registered = true }

func TestPingConnectionInfo_UnregistersAfterThresholdFailures(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	pingErr := errors.New("connection refused")
	for i := 0; i < ConnectionInfoPingThreshold; i++ {
		mock.ExpectPing().WillReturnError(pingErr)
	}

	toggler := &mockToggler{registered: true}
	state := &CIPingState{}

	for i := 0; i < ConnectionInfoPingThreshold; i++ {
		PingConnectionInfo(context.Background(), db, toggler, state)
	}

	require.False(t, toggler.IsRegistered(), "metric should be unregistered after %d consecutive failures", ConnectionInfoPingThreshold)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPingConnectionInfo_ReregistersAfterThresholdSuccesses(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	pingErr := errors.New("connection refused")
	for i := 0; i < ConnectionInfoPingThreshold; i++ {
		mock.ExpectPing().WillReturnError(pingErr)
	}
	for i := 0; i < ConnectionInfoPingThreshold; i++ {
		mock.ExpectPing()
	}

	toggler := &mockToggler{registered: true}
	state := &CIPingState{}

	for i := 0; i < ConnectionInfoPingThreshold*2; i++ {
		PingConnectionInfo(context.Background(), db, toggler, state)
	}

	require.True(t, toggler.IsRegistered(), "metric should be re-registered after %d consecutive successes", ConnectionInfoPingThreshold)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPingConnectionInfo_RemainsRegisteredWhilePingsSucceed(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	const pings = 5
	for i := 0; i < pings; i++ {
		mock.ExpectPing()
	}

	toggler := &mockToggler{registered: true}
	state := &CIPingState{}

	for i := 0; i < pings; i++ {
		PingConnectionInfo(context.Background(), db, toggler, state)
	}

	require.True(t, toggler.IsRegistered(), "metric should remain registered while pings succeed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPingConnectionInfo_ResetsStateWhenTogglerChanges(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	pingErr := errors.New("connection refused")
	for i := 0; i < ConnectionInfoPingThreshold-1; i++ {
		mock.ExpectPing().WillReturnError(pingErr)
	}
	mock.ExpectPing() // first ping with new toggler

	toggler1 := &mockToggler{registered: true}
	state := &CIPingState{}

	for i := 0; i < ConnectionInfoPingThreshold-1; i++ {
		PingConnectionInfo(context.Background(), db, toggler1, state)
	}
	require.True(t, toggler1.IsRegistered(), "should not have unregistered yet")
	require.Equal(t, ConnectionInfoPingThreshold-1, state.failures, "failures should have accumulated")

	toggler2 := &mockToggler{registered: true}
	PingConnectionInfo(context.Background(), db, toggler2, state)

	require.Equal(t, 0, state.failures, "failures should reset when toggler changes")
	require.True(t, toggler1.IsRegistered(), "old toggler should be unaffected")
	require.NoError(t, mock.ExpectationsWereMet())
}
