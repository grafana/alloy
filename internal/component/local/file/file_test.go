package file_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/local/file"
	filedetector "github.com/grafana/alloy/internal/filedetector"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
)

func TestFile(t *testing.T) {
	t.Run("Polling change detector", func(t *testing.T) {
		runFileTests(t, filedetector.DetectorPoll)
	})

	t.Run("Event change detector", func(t *testing.T) {
		runFileTests(t, filedetector.DetectorFSNotify)
	})
}

// runFileTests will run a suite of tests with the configured update type.
func runFileTests(t *testing.T, ut filedetector.Detector) {
	newSuiteController := func(t *testing.T, filename string) *componenttest.Controller {
		require.NoError(t, os.WriteFile(filename, []byte("First load!"), 0664))

		tc, err := componenttest.NewControllerFromID(nil, "local.file")
		require.NoError(t, err)
		go func() {
			err := tc.Run(componenttest.TestContext(t), file.Arguments{
				Filename: filename,
				Type:     ut,

				// Pick a polling frequency which is fast enough so that tests finish
				// quickly but not so frequent such that Go struggles to schedule the
				// goroutines of the tests on slower machines.
				PollFrequency: 50 * time.Millisecond,
			})
			require.NoError(t, err)
		}()

		// Swallow the initial exports notification.
		require.NoError(t, tc.WaitExports(time.Second))
		require.Equal(t, file.Exports{
			Content: alloytypes.OptionalSecret{
				IsSecret: false,
				Value:    "First load!",
			},
		}, tc.Exports())
		return tc
	}

	t.Run("Updates to files are detected", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "testfile")
		sc := newSuiteController(t, testFile)

		// Update the file.
		require.NoError(t, os.WriteFile(testFile, []byte("New content!"), 0664))

		require.NoError(t, sc.WaitExports(time.Second))
		require.Equal(t, file.Exports{
			Content: alloytypes.OptionalSecret{
				IsSecret: false,
				Value:    "New content!",
			},
		}, sc.Exports())
	})

	t.Run("Deleted and recreated files are detected", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "testfile")
		sc := newSuiteController(t, testFile)

		// Delete the file, then recreate it with new content.
		require.NoError(t, os.Remove(testFile))
		require.NoError(t, os.WriteFile(testFile, []byte("New content!"), 0664))

		require.NoError(t, sc.WaitExports(time.Second))
		require.Equal(t, file.Exports{
			Content: alloytypes.OptionalSecret{
				IsSecret: false,
				Value:    "New content!",
			},
		}, sc.Exports())
	})
}

// TestFile_ImmediateExports validates that constructing a local.file component
// immediately exports the contents of the file.
func TestFile_ImmediateExports(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "testfile")
	require.NoError(t, os.WriteFile(testFile, []byte("Hello, world!"), 0664))

	tc, err := componenttest.NewControllerFromID(nil, "local.file")
	require.NoError(t, err)
	go func() {
		err := tc.Run(componenttest.TestContext(t), file.Arguments{
			Filename:      testFile,
			Type:          filedetector.DetectorPoll,
			PollFrequency: 1 * time.Hour,
		})
		require.NoError(t, err)
	}()

	require.NoError(t, tc.WaitExports(time.Second))
	require.Equal(t, file.Exports{
		Content: alloytypes.OptionalSecret{
			IsSecret: false,
			Value:    "Hello, world!",
		},
	}, tc.Exports())
}

// TestFile_ExistOnLoad ensures that the configured file must exist on the
// first load of local.file.
func TestFile_ExistOnLoad(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "testfile")

	tc, err := componenttest.NewControllerFromID(nil, "local.file")
	require.NoError(t, err)

	err = tc.Run(canceledContext(t), file.Arguments{
		Filename:      testFile,
		Type:          filedetector.DetectorPoll,
		PollFrequency: 1 * time.Hour,
	})

	var expectErr error = &fs.PathError{}
	require.ErrorAs(t, err, &expectErr)
}

// canceledContext creates a context which is already canceled.
func canceledContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}
