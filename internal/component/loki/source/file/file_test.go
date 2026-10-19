package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/lokitest"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
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
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTests(t, func(t *testing.T, match FileMatch, p lokitest.Pipeline) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to log to.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
		require.NoError(t, err)

		recv1, recv2 := p.NewReceiver(t), p.NewReceiver(t)

		go func() {
			err := ctrl.Run(ctx, Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": f.Name(),
					"foo":      "bar",
				})},
				ForwardTo: []loki.LogsReceiver{recv1, recv2},
				FileMatch: match,
			}, p.Opts)
			require.NoError(t, err)
		}()

		ctrl.WaitRunning(time.Minute)

		_, err = f.Write([]byte("writing some text\n"))
		require.NoError(t, err)

		wantLabelSet := model.LabelSet{
			"filename": model.LabelValue(f.Name()),
			"foo":      "bar",
		}

		entries := recv1.Next(t, 1)
		require.WithinDuration(t, time.Now(), entries[0].Timestamp, 1*time.Second)
		require.Equal(t, "writing some text", entries[0].Line)
		require.Equal(t, wantLabelSet, entries[0].Labels)

		entries = recv2.Next(t, 1)
		require.WithinDuration(t, time.Now(), entries[0].Timestamp, 1*time.Second)
		require.Equal(t, "writing some text", entries[0].Line)
		require.Equal(t, wantLabelSet, entries[0].Labels)
	})
}

func TestUpdateRemoveFileWhileReading(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTests(t, func(t *testing.T, match FileMatch, p lokitest.Pipeline) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to log to.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
		require.NoError(t, err)

		recv := p.NewReceiver(t)

		go func() {
			err := ctrl.Run(ctx, Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": f.Name(),
					"foo":      "bar",
				})},
				ForwardTo: []loki.LogsReceiver{recv},
				FileMatch: match,
			}, p.Opts)
			require.NoError(t, err)
		}()

		ctrl.WaitRunning(time.Minute)

		workerCtx, cancelWorkers := context.WithCancel(ctx)
		var (
			wg            sync.WaitGroup
			backgroundErr error
		)
		wg.Go(func() {
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
		})

		time.Sleep(100 * time.Millisecond)

		err = ctrl.Update(Arguments{
			Targets:   []discovery.Target{},
			ForwardTo: []loki.LogsReceiver{recv},
			FileMatch: match,
		})
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		err = ctrl.Update(Arguments{
			Targets:   []discovery.Target{},
			ForwardTo: []loki.LogsReceiver{recv},
			FileMatch: match,
		})
		require.NoError(t, err)

		cancelWorkers()
		wg.Wait()
		require.NoError(t, backgroundErr)
	})
}

func TestFileWatch(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	runTests(t, func(t *testing.T, match FileMatch, p lokitest.Pipeline) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			// Create file to log to.
			f, err := os.CreateTemp(t.TempDir(), "example")
			require.NoError(t, err)
			defer f.Close()

			ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
			require.NoError(t, err)

			recv := p.NewReceiver(t)

			args := Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": f.Name(),
					"foo":      "bar",
				})},
				ForwardTo: []loki.LogsReceiver{recv},
				FileWatch: FileWatch{
					MinPollFrequency: time.Millisecond * 500,
					MaxPollFrequency: time.Millisecond * 500,
				},
				FileMatch: match,
			}

			var wg sync.WaitGroup
			wg.Go(func() {
				err := ctrl.Run(ctx, args, p.Opts)
				require.NoError(t, err)
			})

			err = ctrl.WaitRunning(time.Minute)
			require.NoError(t, err)

			// Sleep for 600ms to miss the first poll, the next poll should be MaxPollFrequency later.
			time.Sleep(time.Millisecond * 600)
			_, err = f.Write([]byte("writing some text\n"))
			require.NoError(t, err)

			entries := recv.Next(t, 1)
			require.Equal(t, "writing some text", entries[0].Line)

			// Shut down the component.
			cancel()
			wg.Wait()
		})
	})
}

// Test that updating the component does not leak goroutines.
func TestUpdate_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTests(t, func(t *testing.T, match FileMatch, p lokitest.Pipeline) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Create file to tail.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
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
			err := ctrl.Run(ctx, args, p.Opts)
			require.NoError(t, err)
		}()

		ctrl.WaitRunning(time.Minute)

		// Update a bunch of times to ensure that no goroutines get leaked between
		// updates.
		for range 10 {
			err := ctrl.Update(args)
			require.NoError(t, err)
		}
	})
}

func TestTwoTargets(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTests(t, func(t *testing.T, match FileMatch, p lokitest.Pipeline) {
		dataPath := t.TempDir()

		f, err := os.CreateTemp(dataPath, "example")
		require.NoError(t, err)
		f2, err := os.CreateTemp(dataPath, "example2")
		require.NoError(t, err)
		defer f.Close()
		defer f2.Close()

		ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
		require.NoError(t, err)

		recv := p.NewReceiver(t)
		args := Arguments{
			Targets: []discovery.Target{
				discovery.NewTargetFromMap(map[string]string{"__path__": f.Name(), "foo": "bar"}),
				discovery.NewTargetFromMap(map[string]string{"__path__": f2.Name(), "foo": "bar2"}),
			},
			ForwardTo: []loki.LogsReceiver{recv},
			FileMatch: match,
		}

		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			err := ctrl.Run(ctx, args, p.Opts, func(o component.Options) component.Options {
				// Use a data path the test can inspect after the component stopped.
				o.DataPath = dataPath
				return o
			})
			require.NoError(t, err)
		}()

		ctrl.WaitRunning(time.Minute)

		_, err = f.Write([]byte("text\n"))
		require.NoError(t, err)

		_, err = f2.Write([]byte("text2\n"))
		require.NoError(t, err)

		// The files are tailed independently, so the order of the lines is not fixed.
		entries := recv.Next(t, 2)
		require.WithinDuration(t, time.Now(), entries[0].Timestamp, 1*time.Second)
		require.WithinDuration(t, time.Now(), entries[1].Timestamp, 1*time.Second)
		require.ElementsMatch(t, []string{"text", "text2"}, []string{entries[0].Line, entries[1].Line})
		cancel()
		// Verify that positions.yml is written. NOTE: if we didn't wait for it, there would be a race condition between
		// temporary directory being cleaned up and this file being created.
		require.Eventually(t, func() bool {
			if _, err := os.Stat(filepath.Join(dataPath, "positions.yml")); errors.Is(err, os.ErrNotExist) {
				return false
			}
			return true
		}, 5*time.Second, 10*time.Millisecond, "expected positions.yml file to be written eventually")
	})
}

func TestEncoding(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

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

	encodingCases := []struct {
		name     string
		filename string
		encoding string
	}{
		{"CRLF default encoding", "/CRLF/UTF-8.txt", ""},
		{"CRLF UTF-8", "/CRLF/UTF-8.txt", "UTF-8"},
		{"CRLF UTF-16", "/CRLF/UTF-16.txt", "UTF-16"},
		{"CRLF UTF-16 LE", "/CRLF/UTF-16_LE.txt", "UTF-16LE"},
		{"CRLF UTF-16 BE", "/CRLF/UTF-16_BE.txt", "UTF-16BE"},
		{"CRLF UTF-16 LE with BOM", "/CRLF/UTF-16_LE_BOM.txt", "UTF-16"},
		{"CRLF UTF-16 BE with BOM", "/CRLF/UTF-16_BE_BOM.txt", "UTF-16"},
		{"LF default encoding", "/LF/UTF-8.txt", ""},
		{"LF UTF-8", "/LF/UTF-8.txt", "UTF-8"},
		{"LF UTF-16", "/LF/UTF-16.txt", "UTF-16"},
		{"LF UTF-16 LE", "/LF/UTF-16_LE.txt", "UTF-16LE"},
		{"LF UTF-16 BE", "/LF/UTF-16_BE.txt", "UTF-16BE"},
		{"LF UTF-16 LE with BOM", "/LF/UTF-16_LE_BOM.txt", "UTF-16"},
		{"LF UTF-16 BE with BOM", "/LF/UTF-16_BE_BOM.txt", "UTF-16"},
	}

	// Each fixture is stored uncompressed and once per supported
	// decompression format, with the format as its file extension.
	compressionFormats := []CompressionFormat{"", "gz", "z", "bz2"}

	type testCase struct {
		name                string
		filename            string
		encoding            string
		decompressionConfig DecompressionConfig
	}
	var testCases []testCase
	for _, format := range compressionFormats {
		for _, ec := range encodingCases {
			tc := testCase{name: ec.name, filename: ec.filename, encoding: ec.encoding}
			if format != "" {
				tc.name += " (" + string(format) + ")"
				tc.filename += "." + string(format)
				tc.decompressionConfig = DecompressionConfig{Enabled: true, Format: format}
			}
			testCases = append(testCases, tc)
		}
	}

	runTests(t, func(t *testing.T, match FileMatch, p lokitest.Pipeline) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				dataPath := t.TempDir()

				filePath, err := filepath.Abs(filepath.Join("testdata", "encoding", tc.filename))
				require.NoError(t, err)

				// Verify the test data file exists
				_, err = os.Stat(filePath)
				require.NoError(t, err, fmt.Sprintf("%s test file should exist in testdata/encoding/", tc.filename))

				recv := p.NewReceiver(t)
				args := Arguments{
					Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
						"__path__": filePath,
						"source":   "sql_errorlog_real",
					})},
					FileMatch:           match,
					Encoding:            tc.encoding,
					DecompressionConfig: tc.decompressionConfig,
					ForwardTo:           []loki.LogsReceiver{recv},
				}

				ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
				require.NoError(t, err)

				ctx, cancel := context.WithCancel(componenttest.TestContext(t))

				var wg sync.WaitGroup
				wg.Go(func() {
					err := ctrl.Run(ctx, args, p.Opts, func(o component.Options) component.Options {
						// Use a data path the test can inspect after the component stopped.
						o.DataPath = dataPath
						return o
					})
					require.NoError(t, err)
				})

				expectedLabelSet := model.LabelSet{
					"filename": model.LabelValue(filePath),
					"source":   "sql_errorlog_real",
				}

				entries := recv.Next(t, len(expectedLines))

				receivedLines := make([]string, 0, len(entries))
				for _, e := range entries {
					require.WithinDuration(t, time.Now(), e.Timestamp, 1*time.Second)
					require.Equal(t, expectedLabelSet, e.Labels)
					receivedLines = append(receivedLines, e.Line)
				}
				require.Equal(t, expectedLines, receivedLines)

				// Shut down the component before checking for the positions file.
				// That way it will definitely write to the positions file as part of the shutdown.
				cancel()

				// Verify that positions.yml is written. NOTE: if we didn't wait for it,
				// there would be a race condition between temporary directory being
				// cleaned up and this file being created.
				require.Eventually(t, func() bool {
					if _, err := os.Stat(filepath.Join(dataPath, "positions.yml")); errors.Is(err, os.ErrNotExist) {
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
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTests(t, func(t *testing.T, match FileMatch, p lokitest.Pipeline) {
		ctx, cancel := context.WithCancel(componenttest.TestContext(t))

		// Create file to log to.
		dir := t.TempDir()
		f, err := os.Create(filepath.Join(dir, "example"))
		require.NoError(t, err)

		ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
		require.NoError(t, err)

		recv := p.NewReceiver(t)
		runErr := make(chan error, 1)

		t.Cleanup(func() {
			cancel()
			select {
			case err := <-runErr:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				require.FailNow(t, "timed out waiting for ctrl.Run to exit")
			}
		})

		go func() {
			runErr <- ctrl.Run(ctx, Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": f.Name(),
					"foo":      "bar",
				})},
				ForwardTo: []loki.LogsReceiver{recv},
				FileMatch: match,
			}, p.Opts)
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

		entries := recv.Next(t, 1)
		require.WithinDuration(t, time.Now(), entries[0].Timestamp, 1*time.Second)
		require.Equal(t, "writing some text", entries[0].Line)
		require.Equal(t, wantLabelSet, entries[0].Labels)

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

		entries = recv.Next(t, 1)
		require.WithinDuration(t, time.Now(), entries[0].Timestamp, 1*time.Second)
		require.Equal(t, "writing some new text", entries[0].Line)
		require.Equal(t, wantLabelSet, entries[0].Labels)
	})
}

func runTests(t *testing.T, run func(t *testing.T, match FileMatch, p lokitest.Pipeline)) {
	t.Helper()

	for _, m := range []FileMatch{{Enabled: false, SyncPeriod: 10 * time.Second}, {Enabled: true, SyncPeriod: 10 * time.Second}} {
		t.Run(fmt.Sprintf("file match %t", m.Enabled), func(t *testing.T) {
			lokitest.RunWithPipelines(t, func(t *testing.T, p lokitest.Pipeline) {
				run(t, m, p)
			})
		})
	}
}
