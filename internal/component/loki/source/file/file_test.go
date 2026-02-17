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
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func Test_UnmarshalConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected Arguments
	}{
		{
			name: "default",
			config: `
				forward_to = []
				targets = []`,
			expected: Arguments{
				FileWatch: FileWatch{
					MinPollFrequency: 250 * time.Millisecond,
					MaxPollFrequency: 250 * time.Millisecond,
				},
				FileMatch: FileMatch{
					Enabled:    false,
					SyncPeriod: 10 * time.Second,
				},
				OnPositionsFileError: OnPositionsFileErrorRestartBeginning,
				ForwardTo:            []loki.LogsReceiver{},
				Targets:              []discovery.Target{},
			},
		},
		{
			name: "file_match",
			config: `
				forward_to = []
				targets = [
					{__path__ = "/tmp/*.log"},
				]
				file_match {
					enabled = true
					sync_period = "14s"
				}`,
			expected: Arguments{
				FileWatch: FileWatch{
					MinPollFrequency: 250 * time.Millisecond,
					MaxPollFrequency: 250 * time.Millisecond,
				},
				FileMatch: FileMatch{
					Enabled:    true,
					SyncPeriod: 14 * time.Second,
				},
				OnPositionsFileError: OnPositionsFileErrorRestartBeginning,
				ForwardTo:            []loki.LogsReceiver{},
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__path__": "/tmp/*.log",
					}),
				},
			},
		},
		{
			name: "file_match quoted path",
			config: `
				forward_to = []
				targets = [
					{"__path__" = "/tmp/*.log"},
				]
				file_match {
					enabled = true
					sync_period = "14s"
				}`,
			expected: Arguments{
				FileWatch: FileWatch{
					MinPollFrequency: 250 * time.Millisecond,
					MaxPollFrequency: 250 * time.Millisecond,
				},
				FileMatch: FileMatch{
					Enabled:    true,
					SyncPeriod: 14 * time.Second,
				},
				OnPositionsFileError: OnPositionsFileErrorRestartBeginning,
				ForwardTo:            []loki.LogsReceiver{},
				Targets: []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__path__": "/tmp/*.log",
					}),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tt.config), &args)
			require.NoError(t, err)
			require.Equal(t, tt.expected, args)
		})
	}
}

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

		var backgroundErr error
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
					_, backgroundErr = f.Write([]byte("writing some text\nwriting some text2\n"))
					if backgroundErr != nil {
						return
					}
				}
			}
		}()

		time.Sleep(100 * time.Millisecond)

		err = ctrl.Update(Arguments{
			Targets:   []discovery.Target{},
			ForwardTo: []loki.LogsReceiver{ch1},
			FileMatch: match,
		})
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		err = ctrl.Update(Arguments{
			Targets:   []discovery.Target{},
			ForwardTo: []loki.LogsReceiver{ch1},
			FileMatch: match,
		})
		require.NoError(t, err)

		cancelWorkers()
		wg.Wait()
		require.NoError(t, backgroundErr)
	})
}

func TestFileWatch(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	runTests(t, func(t *testing.T, match FileMatch) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
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

			var wg sync.WaitGroup
			wg.Go(func() {
				err := ctrl.Run(ctx, args)
				require.NoError(t, err)
			})

			err = ctrl.WaitRunning(time.Minute)
			require.NoError(t, err)

			// Sleep for 600ms to miss the first poll, the next poll should be MaxPollFrequency later.
			time.Sleep(time.Millisecond * 600)
			_, err = f.Write([]byte("writing some text\n"))
			require.NoError(t, err)

			select {
			case logEntry := <-ch1.Chan():
				require.Equal(t, "writing some text", logEntry.Line)
			case <-time.After(5 * time.Second):
				require.FailNow(t, "failed waiting for log line")
			}

			// Shut down the component.
			cancel()
			wg.Wait()
		})
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

	expectedLines := []string{
		"2025-03-11 11:11:02.58 Server      Microsoft SQL Server 2019 (RTM) - 15.0.2000.5 (X64) ",
		"	Sep 24 2019 13:48:23 ",
		"	Copyright (C) 2019 Microsoft Corporation",
		"	Enterprise Edition (64-bit) on Windows Server 2022 Standard 10.0 <X64> (Build 20348: ) (Hypervisor)",
		"",
		"2025-03-11 11:11:02.71 Server      UTC adjustment: 1:00",
		"2025-03-11 11:11:02.71 Server      (c) Microsoft Corporation.",
		"2025-03-11 11:11:02.72 Server      All rights reserved.",
		"2025-03-11 11:11:02.72 Server      Server process ID is 4708.",
	}

	noDecompress := DecompressionConfig{}
	gzDecompress := DecompressionConfig{
		Enabled: true,
		Format:  "gz",
	}

	testCases := []struct {
		name                string
		filename            string
		encoding            string
		decompressionConfig DecompressionConfig
	}{
		{"CRLF default encoding", "/CRLF/UTF-8.txt", "", noDecompress},
		{"CRLF UTF-8", "/CRLF/UTF-8.txt", "UTF-8", noDecompress},
		{"CRLF UTF-16", "/CRLF/UTF-16.txt", "UTF-16", noDecompress},
		{"CRLF UTF-16 LE", "/CRLF/UTF-16_LE.txt", "UTF-16LE", noDecompress},
		{"CRLF UTF-16 BE", "/CRLF/UTF-16_BE.txt", "UTF-16BE", noDecompress},
		{"CRLF UTF-16 LE with BOM", "/CRLF/UTF-16_LE_BOM.txt", "UTF-16", noDecompress},
		{"CRLF UTF-16 BE with BOM", "/CRLF/UTF-16_BE_BOM.txt", "UTF-16", noDecompress},
		{"LF default encoding", "/LF/UTF-8.txt", "", noDecompress},
		{"LF UTF-8", "/LF/UTF-8.txt", "UTF-8", noDecompress},
		{"LF UTF-16", "/LF/UTF-16.txt", "UTF-16", noDecompress},
		{"LF UTF-16 LE", "/LF/UTF-16_LE.txt", "UTF-16LE", noDecompress},
		{"LF UTF-16 BE", "/LF/UTF-16_BE.txt", "UTF-16BE", noDecompress},
		{"LF UTF-16 LE with BOM", "/LF/UTF-16_LE_BOM.txt", "UTF-16", noDecompress},
		{"LF UTF-16 BE with BOM", "/LF/UTF-16_BE_BOM.txt", "UTF-16", noDecompress},
		{"CRLF default encoding (gzipped)", "/CRLF/UTF-8.txt.gz", "", gzDecompress},
		{"CRLF UTF-8 (gzipped)", "/CRLF/UTF-8.txt.gz", "UTF-8", gzDecompress},
		{"CRLF UTF-16 (gzipped)", "/CRLF/UTF-16.txt.gz", "UTF-16", gzDecompress},
		{"CRLF UTF-16 LE (gzipped)", "/CRLF/UTF-16_LE.txt.gz", "UTF-16LE", gzDecompress},
		{"CRLF UTF-16 BE (gzipped)", "/CRLF/UTF-16_BE.txt.gz", "UTF-16BE", gzDecompress},
		{"CRLF UTF-16 LE with BOM (gzipped)", "/CRLF/UTF-16_LE_BOM.txt.gz", "UTF-16", gzDecompress},
		{"CRLF UTF-16 BE with BOM (gzipped)", "/CRLF/UTF-16_BE_BOM.txt.gz", "UTF-16", gzDecompress},
		{"LF default encoding (gzipped)", "/LF/UTF-8.txt.gz", "", gzDecompress},
		{"LF UTF-8 (gzipped)", "/LF/UTF-8.txt.gz", "UTF-8", gzDecompress},
		{"LF UTF-16 (gzipped)", "/LF/UTF-16.txt.gz", "UTF-16", gzDecompress},
		{"LF UTF-16 LE (gzipped)", "/LF/UTF-16_LE.txt.gz", "UTF-16LE", gzDecompress},
		{"LF UTF-16 BE (gzipped)", "/LF/UTF-16_BE.txt.gz", "UTF-16BE", gzDecompress},
		{"LF UTF-16 LE with BOM (gzipped)", "/LF/UTF-16_LE_BOM.txt.gz", "UTF-16", gzDecompress},
		{"LF UTF-16 BE with BOM (gzipped)", "/LF/UTF-16_BE_BOM.txt.gz", "UTF-16", gzDecompress},
	}

	runTests(t, func(t *testing.T, match FileMatch) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				opts := component.Options{
					Logger:        util.TestAlloyLogger(t),
					Registerer:    prometheus.NewRegistry(),
					OnStateChange: func(e component.Exports) {},
					DataPath:      t.TempDir(),
				}

				filePath, err := filepath.Abs(filepath.Join("testdata", "encoding", tc.filename))
				require.NoError(t, err)

				// Verify the test data file exists
				_, err = os.Stat(filePath)
				require.NoError(t, err, fmt.Sprintf("%s test file should exist in testdata/encoding/", tc.filename))

				ch1 := loki.NewLogsReceiver()
				args := Arguments{
					Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
						"__path__": filePath,
						"source":   "sql_errorlog_real",
					})},
					FileMatch:           match,
					Encoding:            tc.encoding,
					DecompressionConfig: tc.decompressionConfig,
					ForwardTo:           []loki.LogsReceiver{ch1},
				}

				// Create and run the component
				c, err := New(opts, args)
				require.NoError(t, err)

				ctx, cancel := context.WithCancel(componenttest.TestContext(t))

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := c.Run(ctx)
					require.NoError(t, err)
				}()

				expectedLabelSet := model.LabelSet{
					"filename": model.LabelValue(filePath),
					"source":   "sql_errorlog_real",
				}

				// Collect all received log lines
				receivedLines := make([]string, 0)
				timeout := time.After(10 * time.Second)

				// Read for a reasonable amount of time to get several log entries
				readingComplete := false
				for !readingComplete {
					select {
					case logEntry := <-ch1.Chan():
						require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
						require.Equal(t, expectedLabelSet, logEntry.Labels)

						receivedLines = append(receivedLines, logEntry.Line)
						t.Logf("Received log line %d: %q", len(receivedLines), logEntry.Line)

						// Stop after we have enough lines to verify the first few
						if len(receivedLines) >= len(expectedLines) {
							readingComplete = true
						}

					case <-timeout:
						t.Logf("Timeout reached, received %d log lines total", len(receivedLines))
						readingComplete = true
					}
				}

				// Verify we received all log lines
				require.Len(t, receivedLines, len(expectedLines))
				for i := range expectedLines {
					require.Equal(t, expectedLines[i], receivedLines[i], "log line %d should match", i+1)
				}

				// Shut down the component before checking for the positions file.
				// That way it will definitely write to the positions file as part of the shutdown.
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

				wg.Wait()
			})
		}
	})
}

func TestDeleteRecreateFile(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	runTests(t, func(t *testing.T, match FileMatch) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to log to.
		dir := t.TempDir()
		f, err := os.Create(filepath.Join(dir, "example"))
		require.NoError(t, err)

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

		filename := model.LabelValue(f.Name())
		if match.Enabled {
			filename = model.LabelValue(filepath.Join(dir, "example"))
		}

		wantLabelSet := model.LabelSet{
			"filename": filename,
			"foo":      "bar",
		}

		checkMsg(t, ch1, "writing some text", 5*time.Second, wantLabelSet)

		require.NoError(t, f.Close())
		require.NoError(t, os.Remove(f.Name()))

		// Create a file with the same name. Use eventually because of Windows FS can deny access if this test runs too fast.
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			f, err = os.Create(filepath.Join(dir, "example"))
			assert.NoError(collect, err)
		}, 30*time.Second, 100*time.Millisecond)

		defer os.Remove(f.Name())
		defer f.Close()

		_, err = f.Write([]byte("writing some new text\n"))
		require.NoError(t, err)

		checkMsg(t, ch1, "writing some new text", 5*time.Second, wantLabelSet)
	})
}

func runTests(t *testing.T, run func(t *testing.T, match FileMatch)) {
	t.Helper()

	for _, m := range []FileMatch{{Enabled: false, SyncPeriod: 10 * time.Second}, {Enabled: true, SyncPeriod: 10 * time.Second}} {
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
