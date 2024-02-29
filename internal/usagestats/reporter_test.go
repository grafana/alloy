package usagestats

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

func Test_ReportLoop(t *testing.T) {
	// stub
	reportCheckInterval = 100 * time.Millisecond
	reportInterval = time.Second

	var (
		mut          sync.Mutex
		totalReports int
		agentIDs     []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		mut.Lock()
		defer mut.Unlock()

		totalReports++

		var received Report
		require.NoError(t, jsoniter.NewDecoder(r.Body).Decode(&received))
		agentIDs = append(agentIDs, received.UsageStatsID)

		rw.WriteHeader(http.StatusOK)
	}))
	usageStatsURL = server.URL

	r, err := NewReporter(log.NewLogfmtLogger(os.Stdout))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-time.After(6 * time.Second)
		cancel()
	}()
	metricsFunc := func() map[string]interface{} {
		return map[string]interface{}{}
	}
	require.Equal(t, context.Canceled, r.Start(ctx, metricsFunc))

	mut.Lock()
	defer mut.Unlock()

	require.GreaterOrEqual(t, totalReports, 5)
	first := agentIDs[0]
	for _, uid := range agentIDs {
		require.Equal(t, first, uid)
	}
	require.Equal(t, first, r.agentSeed.UID)
}

func Test_NextReport(t *testing.T) {
	fixtures := map[string]struct {
		interval  time.Duration
		createdAt time.Time
		now       time.Time

		next time.Time
	}{
		"createdAt aligned with interval and now": {
			interval:  1 * time.Hour,
			createdAt: time.Unix(0, time.Hour.Nanoseconds()),
			now:       time.Unix(0, 2*time.Hour.Nanoseconds()),
			next:      time.Unix(0, 2*time.Hour.Nanoseconds()),
		},
		"createdAt aligned with interval": {
			interval:  1 * time.Hour,
			createdAt: time.Unix(0, time.Hour.Nanoseconds()),
			now:       time.Unix(0, 2*time.Hour.Nanoseconds()+1),
			next:      time.Unix(0, 3*time.Hour.Nanoseconds()),
		},
		"createdAt not aligned": {
			interval:  1 * time.Hour,
			createdAt: time.Unix(0, time.Hour.Nanoseconds()+18*time.Minute.Nanoseconds()+20*time.Millisecond.Nanoseconds()),
			now:       time.Unix(0, 2*time.Hour.Nanoseconds()+1),
			next:      time.Unix(0, 2*time.Hour.Nanoseconds()+18*time.Minute.Nanoseconds()+20*time.Millisecond.Nanoseconds()),
		},
	}
	for name, f := range fixtures {
		t.Run(name, func(t *testing.T) {
			next := nextReport(f.interval, f.createdAt, f.now)
			require.Equal(t, f.next, next)
		})
	}
}
