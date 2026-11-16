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

	runTests(t, func(t *testing.T, match FileMatch, env lokitest.Env) {
		// Create file to log to.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		recv1, recv2 := env.NewReceiver(t), env.NewReceiver(t)

		env.Run(t, "loki.source.file", Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": f.Name(),
				"foo":      "bar",
			})},
			ForwardTo: []loki.LogsReceiver{recv1, recv2},
			FileMatch: match,
		})

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

	runTests(t, func(t *testing.T, match FileMatch, env lokitest.Env) {
		// Create file to log to.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		recv := env.NewReceiver(t)

		ctrl, _ := env.Run(t, "loki.source.file", Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": f.Name(),
				"foo":      "bar",
			})},
			ForwardTo: []loki.LogsReceiver{recv},
			FileMatch: match,
		})

		workerCtx, cancelWorkers := context.WithCancel(t.Context())
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
	runTests(t, func(t *testing.T, match FileMatch, env lokitest.Env) {
		synctest.Test(t, func(t *testing.T) {
			// Create file to log to.
			f, err := os.CreateTemp(t.TempDir(), "example")
			require.NoError(t, err)
			defer f.Close()

			recv := env.NewReceiver(t)

			env.Run(t, "loki.source.file", Arguments{
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
			})

			// Sleep for 600ms to miss the first poll, the next poll should be MaxPollFrequency later.
			time.Sleep(time.Millisecond * 600)
			_, err = f.Write([]byte("writing some text\n"))
			require.NoError(t, err)

			entries := recv.Next(t, 1)
			require.Equal(t, "writing some text", entries[0].Line)
		})
	})
}

// Test that updating the component does not leak goroutines.
func TestUpdate_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTests(t, func(t *testing.T, match FileMatch, env lokitest.Env) {
		// Create file to tail.
		f, err := os.CreateTemp(t.TempDir(), "example")
		require.NoError(t, err)
		defer f.Close()

		args := Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": f.Name(),
				"foo":      "bar",
			})},
			ForwardTo: []loki.LogsReceiver{},
			FileMatch: match,
		}

		ctrl, _ := env.Run(t, "loki.source.file", args)

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

	runTests(t, func(t *testing.T, match FileMatch, env lokitest.Env) {
		dataPath := t.TempDir()

		f, err := os.CreateTemp(dataPath, "example")
		require.NoError(t, err)
		f2, err := os.CreateTemp(dataPath, "example2")
		require.NoError(t, err)
		defer f.Close()
		defer f2.Close()

		recv := env.NewReceiver(t)
		args := Arguments{
			Targets: []discovery.Target{
				discovery.NewTargetFromMap(map[string]string{"__path__": f.Name(), "foo": "bar"}),
				discovery.NewTargetFromMap(map[string]string{"__path__": f2.Name(), "foo": "bar2"}),
			},
			ForwardTo: []loki.LogsReceiver{recv},
			FileMatch: match,
		}

		_, stop := env.Run(t, "loki.source.file", args, func(o component.Options) component.Options {
			// Use a data path the test can inspect after the component stopped.
			o.DataPath = dataPath
			return o
		})

		_, err = f.Write([]byte("text\n"))
		require.NoError(t, err)

		_, err = f2.Write([]byte("text2\n"))
		require.NoError(t, err)

		// The files are tailed independently, so the order of the lines is not fixed.
		entries := recv.Next(t, 2)
		require.WithinDuration(t, time.Now(), entries[0].Timestamp, 1*time.Second)
		require.WithinDuration(t, time.Now(), entries[1].Timestamp, 1*time.Second)
		require.ElementsMatch(t, []string{"text", "text2"}, []string{entries[0].Line, entries[1].Line})
		stop()
		require.FileExists(t, filepath.Join(dataPath, "positions.yml"))
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

	runTests(t, func(t *testing.T, match FileMatch, env lokitest.Env) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				dataPath := t.TempDir()

				filePath, err := filepath.Abs(filepath.Join("testdata", "encoding", tc.filename))
				require.NoError(t, err)

				// Verify the test data file exists
				_, err = os.Stat(filePath)
				require.NoError(t, err, fmt.Sprintf("%s test file should exist in testdata/encoding/", tc.filename))

				recv := env.NewReceiver(t)
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

				_, stop := env.Run(t, "loki.source.file", args, func(o component.Options) component.Options {
					// Use a data path the test can inspect after the component stopped.
					o.DataPath = dataPath
					return o
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

				stop()
				require.FileExists(t, filepath.Join(dataPath, "positions.yml"))
			})
		}
	})
}

func TestDeleteRecreateFile(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTests(t, func(t *testing.T, match FileMatch, env lokitest.Env) {
		// Create file to log to.
		dir := t.TempDir()
		f, err := os.Create(filepath.Join(dir, "example"))
		require.NoError(t, err)

		recv := env.NewReceiver(t)

		env.Run(t, "loki.source.file", Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": f.Name(),
				"foo":      "bar",
			})},
			ForwardTo: []loki.LogsReceiver{recv},
			FileMatch: match,
		})

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

		// With file match enabled the recreated file may only be picked up on the next sync tick,
		// so wait longer than SyncPeriod.
		entries = recv.NextWithin(t, 1, 2*match.SyncPeriod)
		require.WithinDuration(t, time.Now(), entries[0].Timestamp, 1*time.Second)
		require.Equal(t, "writing some new text", entries[0].Line)
		require.Equal(t, wantLabelSet, entries[0].Labels)
	})
}

func TestComponentV2(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	lokitest.TestV2(t, func(t *testing.T, env lokitest.Env) {
		// newFile creates a file with three lines, 21 bytes in total.
		newFile := func(t *testing.T) *os.File {
			f, err := os.CreateTemp(t.TempDir(), "example")
			require.NoError(t, err)

			_, err = f.WriteString("line 1\nline 2\nline 3\n")
			require.NoError(t, err)
			return f
		}

		args := func(path string, to ...loki.LogsReceiver) Arguments {
			return Arguments{
				Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
					"__path__": path,
					"foo":      "bar",
				})},
				ForwardTo: to,
				FileMatch: FileMatch{Enabled: false, SyncPeriod: 10 * time.Second},
			}
		}

		// position returns the offset the component has recorded for path.
		position := func(t *testing.T, ctrl *componenttest.Controller, path string) int64 {
			c, err := ctrl.GetComponent()
			require.NoError(t, err)

			offset, err := c.(*Component).posFile.Get(path, model.LabelSet{"foo": "bar"}.String())
			require.NoError(t, err)
			return offset
		}

		t.Run("retries entries until forward_to is updated", func(t *testing.T) {
			f := newFile(t)
			defer f.Close()

			stopped := env.NewReceiver(t)
			stopped.Stop()

			ctrl, stop := env.Run(t, "loki.source.file", args(f.Name(), stopped))

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				require.GreaterOrEqual(c, stopped.ConsumeCalls(), 2)
			}, 5*time.Second, 10*time.Millisecond)

			require.Equal(t, int64(0), position(t, ctrl, f.Name()))

			running := env.NewReceiver(t)
			require.NoError(t, ctrl.Update(args(f.Name(), running)))

			entries := running.Next(t, 3)
			require.Equal(t, "line 1", entries[0].Line)
			require.Equal(t, "line 2", entries[1].Line)
			require.Equal(t, "line 3", entries[2].Line)
			require.Empty(t, stopped.Entries())

			stop()
			require.Equal(t, int64(21), position(t, ctrl, f.Name()))
		})

		t.Run("does not lose entries when stopped while retrying", func(t *testing.T) {
			f := newFile(t)
			defer f.Close()

			// Share the data path so the second run reads the positions the first one saved.
			dataPath := t.TempDir()
			withDataPath := func(o component.Options) component.Options {
				o.DataPath = dataPath
				return o
			}

			stopped := env.NewReceiver(t)
			stopped.Stop()

			ctrl, stop := env.Run(t, "loki.source.file", args(f.Name(), stopped), withDataPath)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				require.GreaterOrEqual(c, stopped.ConsumeCalls(), 2)
			}, 5*time.Second, 10*time.Millisecond)

			stop()
			require.Empty(t, stopped.Entries())
			require.Equal(t, int64(0), position(t, ctrl, f.Name()))

			running := env.NewReceiver(t)
			ctrl, stop = env.Run(t, "loki.source.file", args(f.Name(), running), withDataPath)

			entries := running.Next(t, 3)
			require.Equal(t, "line 1", entries[0].Line)
			require.Equal(t, "line 2", entries[1].Line)
			require.Equal(t, "line 3", entries[2].Line)

			stop()
			require.Equal(t, int64(21), position(t, ctrl, f.Name()))
		})

		t.Run("keeps delivering to a healthy consumer while another is stopped", func(t *testing.T) {
			f := newFile(t)
			defer f.Close()

			healthy, stopped := env.NewReceiver(t), env.NewReceiver(t)
			stopped.Stop()

			ctrl, stop := env.Run(t, "loki.source.file", args(f.Name(), healthy, stopped))

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				require.GreaterOrEqual(c, stopped.ConsumeCalls(), 2)
			}, 5*time.Second, 10*time.Millisecond)

			require.NoError(t, ctrl.Update(args(f.Name(), healthy)))

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				require.Equal(c, int64(21), position(t, ctrl, f.Name()))
			}, 5*time.Second, 10*time.Millisecond)

			// The lines are sent as one batch, so the healthy consumer gets the whole batch again on every retry.
			entries := healthy.Entries()
			require.GreaterOrEqual(t, len(entries), 6)
			require.Equal(t, "line 1", entries[0].Line)
			require.Equal(t, "line 2", entries[1].Line)
			require.Equal(t, "line 3", entries[2].Line)
			require.Equal(t, "line 1", entries[len(entries)-3].Line)
			require.Equal(t, "line 2", entries[len(entries)-2].Line)
			require.Equal(t, "line 3", entries[len(entries)-1].Line)
			require.Empty(t, stopped.Entries())

			stop()
			require.Equal(t, int64(21), position(t, ctrl, f.Name()))
		})

		t.Run("drops an entry on an unexpected error without retrying", func(t *testing.T) {
			f := newFile(t)
			defer f.Close()

			recv := env.NewReceiver(t)

			ctrl, stop := env.Run(t, "loki.source.file", args(f.Name(), recv))

			entries := recv.Next(t, 3)
			require.Equal(t, "line 1", entries[0].Line)
			require.Equal(t, "line 2", entries[1].Line)
			require.Equal(t, "line 3", entries[2].Line)

			recv.FailNext(errors.New("unexpected"))
			_, err := f.WriteString("line 4\n")
			require.NoError(t, err)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				require.Equal(c, 2, recv.ConsumeCalls())
			}, 5*time.Second, 10*time.Millisecond)

			// Line 4 was dropped and is the last line, so the position only includes it if the
			// tailer moved past it, otherwise it would stay at 21.
			stop()
			require.Equal(t, int64(28), position(t, ctrl, f.Name()))
			require.Len(t, recv.Entries(), 3)
		})
	})
}

func runTests(t *testing.T, run func(t *testing.T, match FileMatch, env lokitest.Env)) {
	t.Helper()

	for _, m := range []FileMatch{{Enabled: false, SyncPeriod: 10 * time.Second}, {Enabled: true, SyncPeriod: 10 * time.Second}} {
		t.Run(fmt.Sprintf("file match %t", m.Enabled), func(t *testing.T) {
			lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
				run(t, m, env)
			})
		})
	}
}
