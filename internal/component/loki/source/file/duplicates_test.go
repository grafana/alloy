package file

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

func TestDuplicateDetection(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	tests := []struct {
		name string
		// targetsFn receives two file paths and returns the targets.
		targetsFn          func(file1, file2 string) []discovery.Target
		useSameFile        bool
		expectDuplicate    bool
		expectedDiffLabels string
	}{
		{
			name: "unique_files_no_duplicates",
			targetsFn: func(file1, file2 string) []discovery.Target {
				return []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__path__": file1,
						"app":      "myapp",
						"env":      "prod",
					}),
					discovery.NewTargetFromMap(map[string]string{
						"__path__": file2,
						"app":      "myapp",
						"env":      "staging",
					}),
				}
			},
			useSameFile:     false,
			expectDuplicate: false,
		},
		{
			name: "unique_files_with_differing_internal_labels_no_duplicates",
			targetsFn: func(file1, file2 string) []discovery.Target {
				// Internal labels (__meta_*, __internal) have different values but
				// should be filtered out by NonReservedLabelSet(), so these targets
				// point to different files and should not be duplicates.
				return []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__path__":      file1,
						"__meta_source": "source1",
						"__internal":    "a",
						"app":           "myapp",
					}),
					discovery.NewTargetFromMap(map[string]string{
						"__path__":      file2,
						"__meta_source": "source2",
						"__internal":    "b",
						"app":           "myapp",
					}),
				}
			},
			useSameFile:     false,
			expectDuplicate: false,
		},
		{
			name: "same_file_differing_labels_duplicates",
			targetsFn: func(file1, _ string) []discovery.Target {
				// Both targets point to the same file but have different labels.
				// Also includes differing internal labels to verify they don't
				// appear in the differing_labels output.
				return []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"__path__":      file1,
						"__meta_source": "source1",
						"__internal":    "a",
						"app":           "myapp",
						"port":          "8080",
						"container":     "main",
					}),
					discovery.NewTargetFromMap(map[string]string{
						"__path__":      file1,
						"__meta_source": "source2",
						"__internal":    "b",
						"app":           "myapp",
						"port":          "9090",
						"container":     "sidecar",
					}),
				}
			},
			useSameFile:        true,
			expectDuplicate:    true,
			expectedDiffLabels: "container, port", // Only the non-internal labels should be included.
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(componenttest.TestContext(t))
			defer cancel()

			// Create file(s) for the test.
			f1, err := os.CreateTemp(t.TempDir(), "test_file1")
			require.NoError(t, err)
			defer f1.Close()

			file2Path := f1.Name()
			if !tc.useSameFile {
				f2, err := os.CreateTemp(t.TempDir(), "test_file2")
				require.NoError(t, err)
				defer f2.Close()
				file2Path = f2.Name()
			}

			var logBuf syncbuffer.Buffer
			testLogger, err := logging.New(&logBuf, logging.DefaultOptions)
			require.NoError(t, err)

			ctrl, err := componenttest.NewControllerFromID(testLogger, "loki.source.file")
			require.NoError(t, err)

			ch := loki.NewLogsReceiver()

			var args Arguments
			args.SetToDefault()
			args.Targets = tc.targetsFn(f1.Name(), file2Path)
			args.ForwardTo = []loki.LogsReceiver{ch}

			go func() {
				err := ctrl.Run(ctx, args)
				require.NoError(t, err)
			}()

			ctrl.WaitRunning(time.Minute)

			if tc.expectDuplicate {
				// Wait for duplicate warning to appear in logs.
				require.Eventually(t, func() bool {
					return strings.Contains(logBuf.String(), "file has multiple targets with different labels which will cause duplicate log lines")
				}, 5*time.Second, 10*time.Millisecond, "expected duplicate warning in log")

				logOutput := logBuf.String()

				// The log output may have escaped quotes depending on the logger format.
				require.True(t,
					strings.Contains(logOutput, fmt.Sprintf(`differing_labels="%s"`, tc.expectedDiffLabels)) ||
						strings.Contains(logOutput, fmt.Sprintf(`differing_labels=\"%s\"`, tc.expectedDiffLabels)),
					"expected differing labels %q in log, got: %s", tc.expectedDiffLabels, logOutput)

				// Internal labels should NOT appear in differing labels.
				require.NotContains(t, logOutput, "__meta_source",
					"internal labels should not appear in differing labels")
				require.NotContains(t, logOutput, "__internal",
					"internal labels should not appear in differing labels")
			} else {
				// Wait for tailing to start (indicates targets have been processed).
				require.Eventually(t, func() bool {
					return strings.Contains(logBuf.String(), "start tailing file")
				}, 5*time.Second, 10*time.Millisecond, "expected tailing to start")

				// Wait a bit to ensure duplicate detection has run after tailing started.
				time.Sleep(500 * time.Millisecond)

				// Verify no duplicate warning was logged.
				require.NotContains(t, logBuf.String(),
					"file has multiple targets with different labels",
					"unexpected duplicate warning in log")
			}
		})
	}
}
