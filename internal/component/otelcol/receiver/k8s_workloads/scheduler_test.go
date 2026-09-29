package k8s_workloads

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScanScheduling(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		interval, pause, duration time.Duration
		wantWait                  time.Duration
	}{
		{"slow scan", time.Minute, time.Minute, 3 * time.Minute, time.Minute},
		{"interval dominates", 5 * time.Minute, time.Minute, time.Minute, 4 * time.Minute},
		{"pause dominates", time.Minute, 2 * time.Minute, 10 * time.Second, 2 * time.Minute},
		{"disabled pause", time.Minute, 0, 3 * time.Minute, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				completed := make(chan time.Duration)
				starts := make(chan time.Time, 10)
				done := make(chan struct{})
				go func() {
					defer close(done)
					scheduleScans(ctx, SnapshotArguments{Interval: tc.interval, MinIntervalAfterScan: tc.pause}, func() { starts <- time.Now() }, completed)
				}()
				for range 2 {
					<-starts
					time.Sleep(tc.duration)
					synctest.Wait()
					require.Empty(t, starts, "must not queue scans while one is running")
					finished := time.Now()
					completed <- tc.duration
					if tc.wantWait > 0 {
						time.Sleep(tc.wantWait - time.Nanosecond)
						synctest.Wait()
						require.Empty(t, starts, "must respect pause and interval")
						time.Sleep(time.Nanosecond)
					}
					synctest.Wait()
					require.Len(t, starts, 1)
					next := <-starts
					require.Equal(t, tc.wantWait, next.Sub(finished))
					starts <- next
				}
				cancel()
				<-done
			})
		})
	}
}

func TestScanSchedulingCancelDuringPause(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		completed := make(chan time.Duration)
		starts := 0
		done := make(chan struct{})
		go func() {
			defer close(done)
			scheduleScans(ctx, SnapshotArguments{Interval: time.Minute, MinIntervalAfterScan: time.Minute}, func() { starts++ }, completed)
		}()
		completed <- time.Second
		synctest.Wait()
		cancel()
		<-done
		time.Sleep(2 * time.Minute)
		require.Equal(t, 1, starts)
	})
}
