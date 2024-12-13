//go:build windows

package file

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestDeleteRecreateFileNoRetry(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	filename := "example"

	ctx, cancel := context.WithCancel(componenttest.TestContext(t))
	defer cancel()

	// Create file to log to.
	f, err := os.Create(filename)
	require.NoError(t, err)

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.source.file")
	require.NoError(t, err)

	ch1 := loki.NewLogsReceiver()

	go func() {
		err := ctrl.Run(ctx, Arguments{
			Targets: []discovery.Target{{
				"__path__": f.Name(),
				"foo":      "bar",
			}},
			ForwardTo: []loki.LogsReceiver{ch1},
		})
		require.NoError(t, err)
	}()

	ctrl.WaitRunning(time.Minute)

	_, err = f.Write([]byte("writing some text\n"))
	require.NoError(t, err)

	wantLabelSet := model.LabelSet{
		"filename": model.LabelValue(f.Name()),
		"foo":      "bar",
	}

	select {
	case logEntry := <-ch1.Chan():
		require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
		require.Equal(t, "writing some text", logEntry.Line)
		require.Equal(t, wantLabelSet, logEntry.Labels)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	require.NoError(t, f.Close())
	require.NoError(t, os.Remove(f.Name()))

	// Create a file with the same name
	f, err = os.Create(filename)
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	_, err = f.Write([]byte("writing some new text\n"))
	require.NoError(t, err)

	select {
	case <-ch1.Chan():
		t.Fatalf("Unexpected log entry received")
	case <-time.After(2 * time.Second):
		// Test passes if no log entry is received within the timeout
		// This indicates that the log source does not retry reading the file
	}
}
