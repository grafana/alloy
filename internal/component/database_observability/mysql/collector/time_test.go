package collector_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability/mysql/collector"
)

func Test_CalculateWallTime(t *testing.T) {
	t.Run("calculates the timestamp at which an event happened", func(t *testing.T) {
		serverStartTime := 2.0
		timer := collector.SecondsToPicoseconds(2)
		lastUptime := 0.0

		result := collector.CalculateWallTime(serverStartTime, timer, lastUptime)
		require.Equalf(t, 4000.0, result, "got %f, want 4000", result)
	})

	t.Run("calculates the timestamp, taking into account the overflows", func(t *testing.T) {
		serverStartTime := 3.0
		timer := collector.SecondsToPicoseconds(2)
		lastUptime := collector.PicosecondsToSeconds(math.MaxUint64) + 1

		result := collector.CalculateWallTime(serverStartTime, timer, lastUptime)
		require.Equalf(t, 18446749073.709553, result, "got %f, want 18446749073.709553", result)
	})

	t.Run("calculates another timestamp when timer approaches overflow", func(t *testing.T) {
		serverStartTime := 3.0
		timer := float64(math.MaxUint64 - 5)
		lastUptime := collector.PicosecondsToSeconds(math.MaxUint64) + 1

		result := collector.CalculateWallTime(serverStartTime, timer, lastUptime)
		require.Equalf(t, 3.6893491147419106e+10, result, "got %f, want 3.6893491147419106e+10", result)
	})
}

func Test_CalculateNumberOfOverflows(t *testing.T) {
	testCases := map[string]struct {
		expected uint64
		uptime   float64
	}{
		"0 overflows": {0, 5},
		"1 overflow":  {1, collector.PicosecondsToSeconds(math.MaxUint64) + 5},
		"2 overflows": {2, collector.PicosecondsToSeconds(math.MaxUint64)*2 + 5},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.EqualValues(t, tc.expected, collector.CalculateNumberOfOverflows(tc.uptime))
		})
	}
}
