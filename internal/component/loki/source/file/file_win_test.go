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

// This test:
// - create a file without retry interval -> successful read
// - delete the file, recreate it -> no read
// - update the component with retry interval -> successful read
// - update the component without retry interval, delete the file, recreate it -> no read
func TestDeleteRecreateFileWindows(t *testing.T) {
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

	checkMsg(t, ch1, "writing some text", 5*time.Second, wantLabelSet)

	require.NoError(t, f.Close())
	require.NoError(t, os.Remove(f.Name()))

	// Create a file with the same name
	f, err = os.Create(filename)
	require.NoError(t, err)

	_, err = f.Write([]byte("writing some new text\n"))
	require.NoError(t, err)

	select {
	case <-ch1.Chan():
		t.Fatalf("Unexpected log entry received")
	case <-time.After(1 * time.Second):
		// Test passes if no log entry is received within the timeout
		// This indicates that the log source does not retry reading the file
	}

	// Start the retry interval
	ctrl.Update(Arguments{
		Targets: []discovery.Target{{
			"__path__": f.Name(),
			"foo":      "bar",
		}},
		ForwardTo:     []loki.LogsReceiver{ch1},
		RetryInterval: 200 * time.Millisecond,
	})

	require.NoError(t, f.Close())
	require.NoError(t, os.Remove(f.Name()))

	// Create a file with the same name
	f, err = os.Create(filename)
	require.NoError(t, err)

	checkMsg(t, ch1, "writing some new text", 1*time.Second, wantLabelSet)

	// Stop the retry interval
	ctrl.Update(Arguments{
		Targets: []discovery.Target{{
			"__path__": f.Name(),
			"foo":      "bar",
		}},
		ForwardTo: []loki.LogsReceiver{ch1},
	})

	require.NoError(t, f.Close())
	require.NoError(t, os.Remove(f.Name()))

	// Create a file with the same name
	f, err = os.Create(filename)
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	_, err = f.Write([]byte("writing some new new text\n"))
	require.NoError(t, err)

	select {
	case <-ch1.Chan():
		t.Fatalf("Unexpected log entry received")
	case <-time.After(1 * time.Second):
		// Test passes if no log entry is received within the timeout
		// This indicates that the log source does not retry reading the file
	}
}
