//go:build windows

package windowsevent

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	"github.com/grafana/alloy/internal/loki/util"
)

func TestBookmarkUpdate(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	var loggerName = "alloy_test"
	_ = eventlog.InstallAsEventCreate(loggerName, eventlog.Info|eventlog.Warning|eventlog.Error)
	wlog, err := eventlog.Open(loggerName)
	require.NoError(t, err)

	dirPath := "bookmarktest"
	filePath := path.Join(dirPath, "bookmark.xml")
	require.NoError(t, os.MkdirAll(path.Dir(filePath), 700))
	defer func() {
		require.NoError(t, os.RemoveAll(dirPath))
	}()

	scrapeConfig := &scrapeconfig.WindowsEventsTargetConfig{
		Locale:               0,
		EventlogName:         "Application",
		Query:                "*",
		UseIncomingTimestamp: false,
		BookmarkPath:         filePath,
		PollInterval:         10 * time.Millisecond,
		ExcludeEventData:     false,
		ExcludeEventMessage:  false,
		ExcludeUserData:      false,
		Labels:               util.MapToModelLabelSet(map[string]string{"job": "windows"}),
	}
	handle := &handler{handler: make(chan loki.Entry)}
	winTarget, err := NewTarget(log.NewLogfmtLogger(os.Stderr), handle, nil, scrapeConfig, 1000*time.Millisecond)
	require.NoError(t, err)

	tm := time.Now().Format(time.RFC3339Nano)
	err = wlog.Info(2, tm)
	require.NoError(t, err)

	select {
	case e := <-handle.handler:
		require.Equal(t, model.LabelValue("windows"), e.Labels["job"])
	case <-time.After(3 * time.Second):
		require.FailNow(t, "failed waiting for event")
	}
	winTarget.Stop()

	require.NoError(t, wlog.Close())

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	// check that only the start because the RecordId changes
	require.Contains(t, string(content), "<BookmarkList>\r\n  <Bookmark Channel='Application' RecordId=")
}
