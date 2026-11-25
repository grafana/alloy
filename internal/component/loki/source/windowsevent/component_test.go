//go:build windows

package windowsevent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestEventLogger(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	var loggerName = "alloy_test"
	//Setup Windows Event log with the log source name and logging levels
	_ = eventlog.InstallAsEventCreate(loggerName, eventlog.Info|eventlog.Warning|eventlog.Error)
	wlog, err := eventlog.Open(loggerName)
	require.NoError(t, err)
	dataPath := t.TempDir()
	rec := loki.NewLogsReceiver()
	c, err := New(component.Options{
		ID:       "loki.source.windowsevent.test",
		Logger:   logging.NewSlogNop(),
		DataPath: dataPath,
		OnStateChange: func(e component.Exports) {

		},
		Registerer: prometheus.DefaultRegisterer,
		Tracer:     nil,
	}, Arguments{
		Locale:               0,
		EventLogName:         "Application",
		XPathQuery:           "*",
		BookmarkPath:         "",
		PollInterval:         10 * time.Millisecond,
		ExcludeEventData:     false,
		ExcludeUserdata:      false,
		ExcludeEventMessage:  false,
		UseIncomingTimestamp: false,
		ForwardTo:            []loki.LogsReceiver{rec},
		Labels:               map[string]string{"job": "windows"},
	})
	require.NoError(t, err)
	found := false
	stop := componenttest.RunInBackground(t, c.Run)
	defer stopAndDrain(stop, rec)
	tm := time.Now().Format(time.RFC3339Nano)
	err = wlog.Info(2, tm)
	require.NoError(t, err)
	select {
	case <-time.After(10 * time.Second):
		require.FailNow(t, "timed out waiting for the event")
	case e := <-rec.Chan():
		require.Equal(t, model.LabelValue("windows"), e.Labels["job"])
		if strings.Contains(e.Line, tm) {
			found = true
			break
		}
	}
	require.True(t, found)
}

// stopAndDrain calls stop while reading from rec. Waiting for the component
// to stop matters for goleak: on shutdown the component saves the bookmark
// file, which can be slow enough on Windows for goleak to report the
// still-running goroutines as leaked. Shutdown also forwards pending entries,
// so without a reader stop could block until loki.DefaultDrainTimeout.
func stopAndDrain(stop func(), rec loki.LogsReceiver) {
	quit, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-rec.Chan():
			case <-quit:
				return
			}
		}
	}()
	defer func() {
		close(quit)
		<-done
	}()

	stop()
}

func TestLegacyBookmarkConversion(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	bookmarkText := `<BookmarkList>
  <Bookmark Channel='Application' RecordId='415060' IsCurrent='true'/>
</BookmarkList>`
	var loggerName = "agent_test"
	//Setup Windows Event log with the log source name and logging levels
	_ = eventlog.InstallAsEventCreate(loggerName, eventlog.Info|eventlog.Warning|eventlog.Error)
	dataPath := t.TempDir()
	legacyPath := filepath.Join(t.TempDir(), "legacy.xml")
	err := os.WriteFile(legacyPath, []byte(bookmarkText), 644)
	require.NoError(t, err)
	rec := loki.NewLogsReceiver()
	c, err := New(component.Options{
		ID:       "loki.source.windowsevent.test",
		Logger:   logging.NewSlogNop(),
		DataPath: dataPath,
		OnStateChange: func(e component.Exports) {

		},
		Registerer: prometheus.DefaultRegisterer,
		Tracer:     nil,
	}, Arguments{
		Locale:               0,
		EventLogName:         "Application",
		XPathQuery:           "*",
		BookmarkPath:         "",
		PollInterval:         10 * time.Millisecond,
		ExcludeEventData:     false,
		ExcludeUserdata:      false,
		ExcludeEventMessage:  false,
		UseIncomingTimestamp: false,
		ForwardTo:            []loki.LogsReceiver{rec},
		Labels:               map[string]string{"job": "windows"},
		LegacyBookmarkPath:   legacyPath,
	})
	require.NoError(t, err)
	dd, _ := os.ReadFile(c.args.BookmarkPath)
	// The New function will convert via calling update.
	require.True(t, string(dd) == bookmarkText)

	// Run the component and cancel it to stop the target that was started in New()
	stopAndDrain(componenttest.RunInBackground(t, c.Run), rec)
}
