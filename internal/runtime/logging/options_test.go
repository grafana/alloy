package logging

import (
	"testing"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

// stubIsWindowsService replaces isWindowsService for the duration of the test
// and restores the original on cleanup.
func stubIsWindowsService(t *testing.T, value bool) {
	t.Helper()
	orig := isWindowsService
	t.Cleanup(func() { isWindowsService = orig })
	isWindowsService = func() bool { return value }
}

// TestOptions_EndToEnd drives the real syntax decoder to verify that:
//   - when `destination` is omitted, the platform-appropriate default is used
//     (windows_event_log when running as a service, stderr otherwise);
//   - when the user sets `destination` explicitly, that value wins regardless
//     of the platform;
//   - an unrecognized destination is rejected at decode time.
func TestOptions_EndToEnd(t *testing.T) {
	tests := []struct {
		name             string
		runningAsService bool
		config           string
		want             LogDestination
		wantErr          bool
	}{
		{
			name:             "destination omitted on service uses windows_event_log",
			runningAsService: true,
			config:           `level = "info"`,
			want:             LogDestinationWindowsEventLog,
		},
		{
			name:             "destination omitted off service uses stderr",
			runningAsService: false,
			config:           `level = "info"`,
			want:             LogDestinationStderr,
		},
		{
			name:             "user explicit stderr overrides service default",
			runningAsService: true,
			config:           `destination = "stderr"`,
			want:             LogDestinationStderr,
		},
		{
			name:             "user explicit windows_event_log overrides non-service default",
			runningAsService: false,
			config:           `destination = "windows_event_log"`,
			want:             LogDestinationWindowsEventLog,
		},
		{
			name:             "user picks stderr off service (matches default)",
			runningAsService: false,
			config:           `destination = "stderr"`,
			want:             LogDestinationStderr,
		},
		{
			name:             "user picks windows_event_log on service (matches default)",
			runningAsService: true,
			config:           `destination = "windows_event_log"`,
			want:             LogDestinationWindowsEventLog,
		},
		{
			name:    "unrecognized destination is rejected",
			config:  `destination = "not_a_destination"`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubIsWindowsService(t, tc.runningAsService)

			var o Options
			err := syntax.Unmarshal([]byte(tc.config), &o)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, o.Destination)
		})
	}
}

func TestOptionsDefaultEnablesRateLimiting(t *testing.T) {
	var o Options
	o.SetToDefault()
	require.NotNil(t, o.RateLimiting)
	require.True(t, o.RateLimiting.Enabled)
	require.Equal(t, time.Second, o.RateLimiting.Tick)
	require.Equal(t, uint64(10), o.RateLimiting.Threshold)
	require.Equal(t, 0.0, o.RateLimiting.Rate)
	require.Equal(t, 1000, o.RateLimiting.MaxSignatures)
}

func TestRateLimitingValidate(t *testing.T) {
	valid := RateLimitingOptions{Enabled: true, Tick: time.Second, Threshold: 10, Rate: 0, MaxSignatures: 1000}
	require.NoError(t, valid.Validate())
	require.NoError(t, RateLimitingOptions{Enabled: false}.Validate())
	for _, m := range []func(*RateLimitingOptions){
		func(o *RateLimitingOptions) { o.Tick = 0 },
		func(o *RateLimitingOptions) { o.Threshold = 0 },
		func(o *RateLimitingOptions) { o.Rate = -0.1 },
		func(o *RateLimitingOptions) { o.Rate = 1.1 },
		func(o *RateLimitingOptions) { o.MaxSignatures = 0 },
	} {
		bad := valid
		m(&bad)
		require.Error(t, bad.Validate())
	}
}
