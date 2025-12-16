package collector

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestErrorLogsCollector_ExtractTransactionRollback(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		sqlstate        string
		message         string
		detail          string
		expectedLock    string
		expectedTuple   string
		expectedTimeout string
	}{
		{
			name:          "deadlock with tuple location",
			sqlstate:      "40P01",
			message:       "deadlock detected",
			detail:        "Process 12345 waits for ShareLock on tuple (0,1) of relation 12345",
			expectedLock:  "ShareLock",
			expectedTuple: "0,1",
		},
		{
			name:          "deadlock with exclusive lock",
			sqlstate:      "40P01",
			message:       "deadlock detected",
			detail:        "Process 67890 waits for ExclusiveLock on tuple (1,234) of relation 67890",
			expectedLock:  "ExclusiveLock",
			expectedTuple: "1,234",
		},
		{
			name:            "lock timeout",
			sqlstate:        "55P03",
			message:         "could not obtain lock on relation",
			detail:          "",
			expectedTimeout: "lock_timeout",
		},
		{
			name:     "serialization failure",
			sqlstate: "40001",
			message:  "could not serialize access due to concurrent update",
			detail:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
				Message:       tt.message,
				Detail:        tt.detail,
			}

			collector.extractInsights(parsed)

			if tt.expectedLock != "" {
				require.Equal(t, tt.expectedLock, parsed.LockType, "lock type should be extracted")
			}
			if tt.expectedTuple != "" {
				require.Equal(t, tt.expectedTuple, parsed.TupleLocation, "tuple location should be extracted")
			}
			if tt.expectedTimeout != "" {
				require.Equal(t, tt.expectedTimeout, parsed.TimeoutType, "timeout type should be extracted")
			}
		})
	}
}

func TestErrorLogsCollector_ExtractAuthFailure(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		sqlstate        string
		message         string
		detail          string
		expectedAuth    string
		expectedHBALine string
	}{
		{
			name:            "password auth failed",
			sqlstate:        "28P01",
			message:         `password authentication failed for user "myuser"`,
			detail:          `Connection matched pg_hba.conf line 95: "host all all 0.0.0.0/0 md5"`,
			expectedAuth:    "password",
			expectedHBALine: "95",
		},
		{
			name:            "md5 auth failed",
			sqlstate:        "28000",
			message:         `md5 authentication failed for user "admin"`,
			detail:          `Connection matched pg_hba.conf line 10: "local all all md5"`,
			expectedAuth:    "md5",
			expectedHBALine: "10",
		},
		{
			name:            "scram-sha-256 auth failed",
			sqlstate:        "28P01",
			message:         `scram-sha-256 authentication failed for user "testuser"`,
			detail:          `Connection matched pg_hba.conf line 5: "hostssl all all 0.0.0.0/0 scram-sha-256"`,
			expectedAuth:    "scram-sha-256",
			expectedHBALine: "5",
		},
		{
			name:            "custom hba file with full path",
			sqlstate:        "28P01",
			message:         `password authentication failed for user "dbuser"`,
			detail:          `Connection matched file "/etc/postgresql/pg_hba_cluster.conf" line 42: "host all all 0.0.0.0/0 md5"`,
			expectedAuth:    "password",
			expectedHBALine: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
				Message:       tt.message,
				Detail:        tt.detail,
			}

			collector.extractInsights(parsed)

			if tt.expectedAuth != "" {
				require.Equal(t, tt.expectedAuth, parsed.AuthMethod, "auth method should be extracted")
			}
			if tt.expectedHBALine != "" {
				require.Equal(t, tt.expectedHBALine, parsed.HBALineNumber, "HBA line number should be extracted")
			}
		})
	}
}

func TestErrorLogsCollector_ExtractTimeoutError(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)

	tests := []struct {
		name            string
		sqlstate        string
		message         string
		expectedTimeout string
	}{
		{
			name:            "statement timeout",
			sqlstate:        "57014",
			message:         "canceling statement due to statement timeout",
			expectedTimeout: "statement_timeout",
		},
		{
			name:            "lock timeout",
			sqlstate:        "57014",
			message:         "canceling statement due to lock timeout",
			expectedTimeout: "lock_timeout",
		},
		{
			name:            "user cancel",
			sqlstate:        "57014",
			message:         "canceling statement due to user request",
			expectedTimeout: "user_cancel",
		},
		{
			name:            "idle in transaction timeout",
			sqlstate:        "57014",
			message:         "terminating connection due to idle_in_transaction_session_timeout",
			expectedTimeout: "idle_in_transaction_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := &ParsedError{
				SQLStateCode:  tt.sqlstate,
				SQLStateClass: tt.sqlstate[:2],
				Message:       tt.message,
			}

			collector.extractInsights(parsed)

			if tt.expectedTimeout != "" {
				require.Equal(t, tt.expectedTimeout, parsed.TimeoutType, "timeout type should be extracted")
			}
		})
	}
}
