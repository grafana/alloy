//go:build !race

package file

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/text/encoding/unicode"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
)

func TestComponent(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	runTests(t, func(t *testing.T, match FileMatch) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to log to.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(logging.NewNop(), "loki.source.file")
		require.NoError(t, err)

		ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()

		go func() {
			err := ctrl.Run(ctx, Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": f.Name(),
					"foo":      "bar",
				})},
				ForwardTo: []loki.LogsReceiver{ch1, ch2},
				FileMatch: match,
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

		for i := 0; i < 2; i++ {
			select {
			case logEntry := <-ch1.Chan():
				require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
				require.Equal(t, "writing some text", logEntry.Line)
				require.Equal(t, wantLabelSet, logEntry.Labels)
			case logEntry := <-ch2.Chan():
				require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
				require.Equal(t, "writing some text", logEntry.Line)
				require.Equal(t, wantLabelSet, logEntry.Labels)
			case <-time.After(5 * time.Second):
				require.FailNow(t, "failed waiting for log line")
			}
		}
	})
}

func TestUpdateRemoveFileWhileReading(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	runTests(t, func(t *testing.T, match FileMatch) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to log to.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(logging.NewNop(), "loki.source.file")
		require.NoError(t, err)

		ch1 := loki.NewLogsReceiver()

		go func() {
			err := ctrl.Run(ctx, Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": f.Name(),
					"foo":      "bar",
				})},
				ForwardTo: []loki.LogsReceiver{ch1},
				FileMatch: match,
			})
			require.NoError(t, err)
		}()

		ctrl.WaitRunning(time.Minute)

		workerCtx, cancelWorkers := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(2)

		// Start a goroutine that reads from the channel until cancellation
		go func() {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-ch1.Chan():
					// Just consume the messages
				}
			}
		}()

		go func() {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				default:
					_, err = f.Write([]byte("writing some text\nwriting some text2\n"))
					require.NoError(t, err)
				}
			}
		}()

		time.Sleep(100 * time.Millisecond)

		err = ctrl.Update(Arguments{
			Targets:   []discovery.Target{},
			ForwardTo: []loki.LogsReceiver{ch1},
		})
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		err = ctrl.Update(Arguments{
			Targets:   []discovery.Target{},
			ForwardTo: []loki.LogsReceiver{ch1},
		})
		require.NoError(t, err)

		cancelWorkers()
		wg.Wait()
	})
}

func TestFileWatch(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	runTests(t, func(t *testing.T, match FileMatch) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))

		// Create file to log to.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(logging.NewNop(), "loki.source.file")
		require.NoError(t, err)

		ch1 := loki.NewLogsReceiver()

		args := Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": f.Name(),
				"foo":      "bar",
			})},
			ForwardTo: []loki.LogsReceiver{ch1},
			FileWatch: FileWatch{
				MinPollFrequency: time.Millisecond * 500,
				MaxPollFrequency: time.Millisecond * 500,
			},
			FileMatch: match,
		}

		go func() {
			err := ctrl.Run(ctx, args)
			require.NoError(t, err)
		}()

		err = ctrl.WaitRunning(time.Minute)
		require.NoError(t, err)

		timeBeforeWriting := time.Now()

		// Sleep for 600ms to miss the first poll, the next poll should be MaxPollFrequency later.
		time.Sleep(time.Millisecond * 600)

		_, err = f.Write([]byte("writing some text\n"))
		require.NoError(t, err)

		select {
		case logEntry := <-ch1.Chan():
			require.Greater(t, time.Since(timeBeforeWriting), 1*time.Second)
			require.WithinDuration(t, time.Now(), timeBeforeWriting, 2*time.Second)
			require.Equal(t, "writing some text", logEntry.Line)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "failed waiting for log line")
		}

		// Shut down the component.
		cancel()

		// Wait to make sure that all go routines stopped.
		time.Sleep(args.FileWatch.MaxPollFrequency)
	})
}

// Test that updating the component does not leak goroutines.
func TestUpdate_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	runTests(t, func(t *testing.T, match FileMatch) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to tail.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(logging.NewNop(), "loki.source.file")
		require.NoError(t, err)

		args := Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": f.Name(),
				"foo":      "bar",
			})},
			ForwardTo: []loki.LogsReceiver{},
			FileMatch: match,
		}

		go func() {
			err := ctrl.Run(ctx, args)
			require.NoError(t, err)
		}()

		ctrl.WaitRunning(time.Minute)

		// Update a bunch of times to ensure that no goroutines get leaked between
		// updates.
		for i := 0; i < 10; i++ {
			err := ctrl.Update(args)
			require.NoError(t, err)
		}
	})
}

func TestTwoTargets(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	// FIXME use test ctrl
	runTests(t, func(t *testing.T, match FileMatch) {
		// Create opts for component
		opts := component.Options{
			Logger:        util.TestAlloyLogger(t),
			Registerer:    prometheus.NewRegistry(),
			OnStateChange: func(e component.Exports) {},
			DataPath:      t.TempDir(),
		}

		f, err := os.CreateTemp(opts.DataPath, "example")
		if err != nil {
			log.Fatal(err)
		}
		f2, err := os.CreateTemp(opts.DataPath, "example2")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		defer f2.Close()

		ch1 := loki.NewLogsReceiver()
		args := Arguments{
			Targets: []discovery.Target{
				discovery.NewTargetFromMap(map[string]string{"__path__": f.Name(), "foo": "bar"}),
				discovery.NewTargetFromMap(map[string]string{"__path__": f2.Name(), "foo": "bar2"}),
			},
			ForwardTo: []loki.LogsReceiver{ch1},
			FileMatch: match,
		}

		c, err := New(opts, args)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		go c.Run(ctx)
		time.Sleep(100 * time.Millisecond)

		_, err = f.Write([]byte("text\n"))
		require.NoError(t, err)

		_, err = f2.Write([]byte("text2\n"))
		require.NoError(t, err)

		foundF1, foundF2 := false, false
		for i := 0; i < 2; i++ {
			select {
			case logEntry := <-ch1.Chan():
				require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
				switch logEntry.Line {
				case "text":
					foundF1 = true
				case "text2":
					foundF2 = true
				}

			case <-time.After(5 * time.Second):
				require.FailNow(t, "failed waiting for log line")
			}
		}
		require.True(t, foundF1)
		require.True(t, foundF2)
		cancel()
		// Verify that positions.yml is written. NOTE: if we didn't wait for it, there would be a race condition between
		// temporary directory being cleaned up and this file being created.
		require.Eventually(t, func() bool {
			if _, err := os.Stat(filepath.Join(opts.DataPath, "positions.yml")); errors.Is(err, os.ErrNotExist) {
				return false
			}
			return true
		}, 5*time.Second, 10*time.Millisecond, "expected positions.yml file to be written eventually")
	})

}

func TestEncoding(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	// FIXME use test ctrl
	runTests(t, func(t *testing.T, match FileMatch) {
		// Create opts for component
		opts := component.Options{
			Logger:        util.TestAlloyLogger(t),
			Registerer:    prometheus.NewRegistry(),
			OnStateChange: func(e component.Exports) {},
			DataPath:      t.TempDir(),
		}

		// Create a file to write to and set up the component's Arguments.
		f, err := os.CreateTemp(opts.DataPath, "example")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		ch1 := loki.NewLogsReceiver()
		args := Arguments{
			Targets:   []discovery.Target{discovery.NewTargetFromMap(map[string]string{"__path__": f.Name(), "lbl1": "val1"})},
			Encoding:  "UTF-16BE",
			ForwardTo: []loki.LogsReceiver{ch1},
			FileMatch: match,
		}

		// Create and run the component.
		c, err := New(opts, args)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		go c.Run(ctx)
		require.Eventually(t, func() bool { return c.DebugInfo() != nil }, 500*time.Millisecond, 20*time.Millisecond)

		// Write a UTF-16BE encoded byte slice to the file.
		utf16Encoder := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
		utf16Bytes, err := utf16Encoder.Bytes([]byte("hello world!\n"))
		require.Nil(t, err)

		_, err = f.Write(utf16Bytes)
		require.Nil(t, err)

		// Make sure the log was received successfully with the correct format.
		select {
		case logEntry := <-ch1.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "hello world!�", logEntry.Line)

		case <-time.After(5 * time.Second):
			require.FailNow(t, "failed waiting for log line")
		}

		// Shut down the component
		cancel()

		// Verify that positions.yml is written. NOTE: if we didn't wait for it,
		// there would be a race condition between temporary directory being
		// cleaned up and this file being created.
		require.Eventually(t, func() bool {
			if _, err := os.Stat(filepath.Join(opts.DataPath, "positions.yml")); errors.Is(err, os.ErrNotExist) {
				return false
			}
			return true
		}, 5*time.Second, 10*time.Millisecond, "expected positions.yml file to be written eventually")
	})

}

func TestDeleteRecreateFile(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	runTests(t, func(t *testing.T, match FileMatch) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to log to.
		f, err := os.Create("example")
		require.NoError(t, err)
		defer os.Remove(f.Name())
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(logging.NewNop(), "loki.source.file")
		require.NoError(t, err)

		ch1 := loki.NewLogsReceiver()

		go func() {
			err := ctrl.Run(ctx, Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": f.Name(),
					"foo":      "bar",
				})},
				ForwardTo: []loki.LogsReceiver{ch1},
				FileMatch: match,
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

		// Create a file with the same name. Use eventually because of Windows FS can deny access if this test runs too fast.
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			f, err = os.Create("example")
			assert.NoError(collect, err)
		}, 30*time.Second, 100*time.Millisecond)

		_, err = f.Write([]byte("writing some new text\n"))
		require.NoError(t, err)

		checkMsg(t, ch1, "writing some new text", 5*time.Second, wantLabelSet)
	})

}

func runTests(t *testing.T, run func(t *testing.T, match FileMatch)) {
	t.Helper()

	for _, m := range []FileMatch{{Enabled: false}, {Enabled: true, SyncPeriod: 10 * time.Second}} {
		t.Run(fmt.Sprintf("file match %t", m.Enabled), func(t *testing.T) {
			run(t, m)
		})
	}
}

func checkMsg(t *testing.T, ch loki.LogsReceiver, msg string, timeout time.Duration, labelSet model.LabelSet) {
	select {
	case logEntry := <-ch.Chan():
		require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
		require.Equal(t, msg, logEntry.Line)
		require.Equal(t, labelSet, logEntry.Labels)
	case <-time.After(timeout):
		require.FailNow(t, "failed waiting for log line")
	}
}
