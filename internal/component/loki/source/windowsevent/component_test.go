//go:build windows

package windowsevent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sys/windows/svc/eventlog"
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
		Logger:   util.TestAlloyLogger(t),
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
	ctx := t.Context()
	ctx, cancelFunc := context.WithTimeout(ctx, 10*time.Second)
	found := false
	go c.Run(ctx)
	tm := time.Now().Format(time.RFC3339Nano)
	err = wlog.Info(2, tm)
	require.NoError(t, err)
	select {
	case <-ctx.Done():
		// Fail!
		require.True(t, false)
	case e := <-rec.Chan():
		require.Equal(t, model.LabelValue("windows"), e.Labels["job"])
		if strings.Contains(e.Line, tm) {
			found = true
			break
		}
	}
	cancelFunc()
	require.True(t, found)
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
		Logger:   util.TestAlloyLogger(t),
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
	ctx := t.Context()
	ctx, cancelFunc := context.WithTimeout(ctx, 10*time.Second)
	go c.Run(ctx)
	cancelFunc()
}
