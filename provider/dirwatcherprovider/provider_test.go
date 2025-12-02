// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dirwatcherprovider

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

// testDebouncePeriod is a short debounce period for tests (50ms instead of 5s)
const testDebouncePeriod = 50 * time.Millisecond

const schemePrefix = schemeName + ":"

// Helper function to create a provider for tests
func createProvider() confmap.Provider {
	return NewFactory().Create(confmaptest.NewNopProviderSettings())
}

// Helper function to get absolute path
func absolutePath(t *testing.T, relativePath string) string {
	dir, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Join(dir, relativePath)
}

func TestScheme(t *testing.T) {
	fp := createProvider()
	assert.Equal(t, "dirwatch", fp.Scheme())
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)

	provider := factory.Create(confmaptest.NewNopProviderSettings())
	require.NotNil(t, provider)
}

func TestValidateProviderScheme(t *testing.T) {
	assert.NoError(t, confmaptest.ValidateProviderScheme(createProvider()))
}

func TestListYAMLFiles(t *testing.T) {
	tests := []struct {
		name          string
		dir           string
		expectedFiles []string
		expectError   bool
	}{
		{
			name: "valid directory with multiple yaml files",
			dir:  filepath.Join("testdata", "valid"),
			expectedFiles: []string{
				filepath.Join("testdata", "valid", "01-receivers.yaml"),
				filepath.Join("testdata", "valid", "02-exporters.yaml"),
				filepath.Join("testdata", "valid", "03-service.yaml"),
			},
			expectError: false,
		},
		{
			name:          "single file directory",
			dir:           filepath.Join("testdata", "single"),
			expectedFiles: []string{filepath.Join("testdata", "single", "config.yaml")},
			expectError:   false,
		},
		{
			name:          "empty directory",
			dir:           filepath.Join("testdata", "empty"),
			expectedFiles: nil, // empty directory returns nil slice
			expectError:   false,
		},
		{
			name:          "non-existent directory",
			dir:           filepath.Join("testdata", "non-existent"),
			expectedFiles: nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := listYAMLFiles(tt.dir)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.expectedFiles == nil {
				assert.Empty(t, files)
			} else {
				assert.Equal(t, tt.expectedFiles, files)
			}
		})
	}
}

func TestListYAMLFilesFiltersNonYAML(t *testing.T) {
	// Create a temp directory with mixed files
	tmpDir := t.TempDir()

	// Create test files
	files := []string{
		"config.yaml",
		"other.yml",
		"readme.txt",
		"script.sh",
		".hidden.yaml", // hidden files should be ignored
	}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("test: value"), 0o600)
		require.NoError(t, err)
	}

	result, err := listYAMLFiles(tmpDir)
	require.NoError(t, err)

	// Should only include .yaml and .yml files, not hidden files
	assert.Len(t, result, 2)
	assert.Contains(t, result, filepath.Join(tmpDir, "config.yaml"))
	assert.Contains(t, result, filepath.Join(tmpDir, "other.yml"))
}

func TestListYAMLFilesSortsAlphabetically(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in non-alphabetical order
	files := []string{"z-last.yaml", "a-first.yaml", "m-middle.yaml"}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("test: value"), 0o600)
		require.NoError(t, err)
	}

	result, err := listYAMLFiles(tmpDir)
	require.NoError(t, err)

	expected := []string{
		filepath.Join(tmpDir, "a-first.yaml"),
		filepath.Join(tmpDir, "m-middle.yaml"),
		filepath.Join(tmpDir, "z-last.yaml"),
	}
	assert.Equal(t, expected, result)
}

func TestMergeYAMLFiles(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		expectError bool
		validate    func(t *testing.T, result map[string]any)
	}{
		{
			name:        "merge multiple valid files",
			dir:         filepath.Join("testdata", "valid"),
			expectError: false,
			validate: func(t *testing.T, result map[string]any) {
				// Check receivers from 01-receivers.yaml
				receivers, ok := result["receivers"].(map[string]any)
				require.True(t, ok, "receivers should exist")
				_, ok = receivers["otlp"]
				assert.True(t, ok, "otlp receiver should exist")

				// Check exporters from 02-exporters.yaml
				exporters, ok := result["exporters"].(map[string]any)
				require.True(t, ok, "exporters should exist")
				_, ok = exporters["debug"]
				assert.True(t, ok, "debug exporter should exist")

				// Check service from 03-service.yaml
				service, ok := result["service"].(map[string]any)
				require.True(t, ok, "service should exist")
				_, ok = service["pipelines"]
				assert.True(t, ok, "pipelines should exist")
			},
		},
		{
			name:        "single file",
			dir:         filepath.Join("testdata", "single"),
			expectError: false,
			validate: func(t *testing.T, result map[string]any) {
				_, ok := result["receivers"]
				assert.True(t, ok, "receivers should exist")
				_, ok = result["exporters"]
				assert.True(t, ok, "exporters should exist")
				_, ok = result["service"]
				assert.True(t, ok, "service should exist")
			},
		},
		{
			name:        "empty directory returns empty map",
			dir:         filepath.Join("testdata", "empty"),
			expectError: false,
			validate: func(t *testing.T, result map[string]any) {
				assert.Empty(t, result)
			},
		},
		{
			name:        "invalid YAML fails entire load",
			dir:         filepath.Join("testdata", "invalid"),
			expectError: true,
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := listYAMLFiles(tt.dir)
			require.NoError(t, err)

			result, err := mergeYAMLFiles(files)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestMergeYAMLFilesOverrides(t *testing.T) {
	// Create temp directory with files that have overlapping keys
	tmpDir := t.TempDir()

	// First file sets a value (alphabetically first)
	err := os.WriteFile(filepath.Join(tmpDir, "01-base.yaml"), []byte(`
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
`), 0o600)
	require.NoError(t, err)

	// Second file overrides the endpoint (alphabetically second)
	err = os.WriteFile(filepath.Join(tmpDir, "02-override.yaml"), []byte(`
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
`), 0o600)
	require.NoError(t, err)

	files, err := listYAMLFiles(tmpDir)
	require.NoError(t, err)

	result, err := mergeYAMLFiles(files)
	require.NoError(t, err)

	// The override should win (later file in alphabetical order takes precedence)
	receivers := result["receivers"].(map[string]any)
	otlp := receivers["otlp"].(map[string]any)
	protocols := otlp["protocols"].(map[string]any)
	grpc := protocols["grpc"].(map[string]any)
	assert.Equal(t, "0.0.0.0:4317", grpc["endpoint"])
}

func TestRetrieveUnsupportedScheme(t *testing.T) {
	fp := createProvider()
	_, err := fp.Retrieve(context.Background(), "file:/some/path", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not supported by")
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRetrieveEmptyURI(t *testing.T) {
	fp := createProvider()
	_, err := fp.Retrieve(context.Background(), "", nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRetrieveNonExistentDirectory(t *testing.T) {
	fp := createProvider()
	_, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "non-existent"), nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRetrieveValidDirectory(t *testing.T) {
	fp := createProvider()
	ret, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "valid"), nil)
	require.NoError(t, err)

	conf, err := ret.AsConf()
	require.NoError(t, err)

	// Verify merged configuration
	assert.True(t, conf.IsSet("receivers"))
	assert.True(t, conf.IsSet("exporters"))
	assert.True(t, conf.IsSet("service"))

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRetrieveAbsolutePath(t *testing.T) {
	fp := createProvider()
	absPath := absolutePath(t, filepath.Join("testdata", "valid"))
	ret, err := fp.Retrieve(context.Background(), schemePrefix+absPath, nil)
	require.NoError(t, err)

	conf, err := ret.AsConf()
	require.NoError(t, err)

	assert.True(t, conf.IsSet("receivers"))
	assert.True(t, conf.IsSet("exporters"))
	assert.True(t, conf.IsSet("service"))

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRetrieveEmptyDirectory(t *testing.T) {
	fp := createProvider()
	ret, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "empty"), nil)
	require.NoError(t, err)

	conf, err := ret.AsConf()
	require.NoError(t, err)

	// Empty directory should return empty config
	assert.Empty(t, conf.AllKeys())

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRetrieveInvalidYAML(t *testing.T) {
	fp := createProvider()
	_, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "invalid"), nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestRetrieveSingleFile(t *testing.T) {
	fp := createProvider()
	ret, err := fp.Retrieve(context.Background(), schemePrefix+filepath.Join("testdata", "single"), nil)
	require.NoError(t, err)

	conf, err := ret.AsConf()
	require.NoError(t, err)

	assert.True(t, conf.IsSet("receivers"))
	assert.True(t, conf.IsSet("exporters"))
	assert.True(t, conf.IsSet("service"))

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

// =============================================================================
// Phase 3 Tests: File Watching
// =============================================================================

func TestWatcherNotTriggeredWithoutWatcherFunc(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()
	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, nil)
	require.NoError(t, err)

	// Modify a file - should not cause issues since no watcher
	err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: newvalue"), 0o600)
	require.NoError(t, err)

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestWatcherTriggeredOnFileModification(t *testing.T) {
	restore := SetDebouncePeriodForTest(testDebouncePeriod)
	defer restore()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()

	var watcherCalled atomic.Int32
	watcher := func(event *confmap.ChangeEvent) {
		watcherCalled.Add(1)
	}

	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, watcher)
	require.NoError(t, err)

	// Modify the file
	err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: newvalue"), 0o600)
	require.NoError(t, err)

	// Wait for debounce period + buffer for fsnotify event delivery
	assert.Eventually(t, func() bool {
		return watcherCalled.Load() >= 1
	}, 500*time.Millisecond, 10*time.Millisecond, "watcher should be called after file modification")

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestWatcherTriggeredOnFileCreation(t *testing.T) {
	restore := SetDebouncePeriodForTest(testDebouncePeriod)
	defer restore()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()

	var watcherCalled atomic.Int32
	watcher := func(event *confmap.ChangeEvent) {
		watcherCalled.Add(1)
	}

	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, watcher)
	require.NoError(t, err)

	// Create a new file
	err = os.WriteFile(filepath.Join(tmpDir, "new-config.yaml"), []byte("new: config"), 0o600)
	require.NoError(t, err)

	// Wait for debounce period + buffer
	assert.Eventually(t, func() bool {
		return watcherCalled.Load() >= 1
	}, 500*time.Millisecond, 10*time.Millisecond, "watcher should be called after file creation")

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestWatcherTriggeredOnFileDeletion(t *testing.T) {
	restore := SetDebouncePeriodForTest(testDebouncePeriod)
	defer restore()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()

	var watcherCalled atomic.Int32
	watcher := func(event *confmap.ChangeEvent) {
		watcherCalled.Add(1)
	}

	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, watcher)
	require.NoError(t, err)

	// Delete the file
	err = os.Remove(configPath)
	require.NoError(t, err)

	// Wait for debounce period + buffer
	assert.Eventually(t, func() bool {
		return watcherCalled.Load() >= 1
	}, 500*time.Millisecond, 10*time.Millisecond, "watcher should be called after file deletion")

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestWatcherIgnoresNonYAMLFiles(t *testing.T) {
	restore := SetDebouncePeriodForTest(testDebouncePeriod)
	defer restore()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()

	var watcherCalled atomic.Int32
	watcher := func(event *confmap.ChangeEvent) {
		watcherCalled.Add(1)
	}

	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, watcher)
	require.NoError(t, err)

	// Create a non-YAML file - should be ignored
	err = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("some text"), 0o600)
	require.NoError(t, err)

	// Wait longer than debounce period to ensure no notification
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), watcherCalled.Load(), "watcher should not be called for non-YAML files")

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestWatcherDebouncing(t *testing.T) {
	restore := SetDebouncePeriodForTest(testDebouncePeriod)
	defer restore()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()

	var watcherCalled atomic.Int32
	watcher := func(event *confmap.ChangeEvent) {
		watcherCalled.Add(1)
	}

	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, watcher)
	require.NoError(t, err)

	// Make multiple rapid modifications (faster than debounce period)
	for i := range 5 {
		err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"+string(rune('0'+i))), 0o600)
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond) // Much shorter than debounce
	}

	// Wait for debounce period + buffer
	time.Sleep(150 * time.Millisecond)

	// Watcher should only be called once due to debouncing
	calls := watcherCalled.Load()
	assert.Equal(t, int32(1), calls, "watcher should only be called once due to debouncing, got %d calls", calls)

	require.NoError(t, ret.Close(context.Background()))
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestWatcherStopsOnClose(t *testing.T) {
	restore := SetDebouncePeriodForTest(testDebouncePeriod)
	defer restore()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()

	var watcherCalled atomic.Int32
	watcher := func(event *confmap.ChangeEvent) {
		watcherCalled.Add(1)
	}

	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, watcher)
	require.NoError(t, err)

	// Close the retrieved config (stops watching)
	require.NoError(t, ret.Close(context.Background()))

	// Modify the file after close
	err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: newvalue"), 0o600)
	require.NoError(t, err)

	// Wait and ensure watcher was NOT called
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), watcherCalled.Load(), "watcher should not be called after Close")

	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestShutdownStopsAllWatchers(t *testing.T) {
	restore := SetDebouncePeriodForTest(testDebouncePeriod)
	defer restore()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: value"), 0o600)
	require.NoError(t, err)

	fp := createProvider()

	var watcherCalled atomic.Int32
	watcher := func(event *confmap.ChangeEvent) {
		watcherCalled.Add(1)
	}

	ret, err := fp.Retrieve(context.Background(), schemePrefix+tmpDir, watcher)
	require.NoError(t, err)

	// Shutdown the provider
	require.NoError(t, fp.Shutdown(context.Background()))

	// Modify the file after shutdown
	err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("test: newvalue"), 0o600)
	require.NoError(t, err)

	// Wait and ensure watcher was NOT called
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), watcherCalled.Load(), "watcher should not be called after Shutdown")

	// Close should still work without error
	require.NoError(t, ret.Close(context.Background()))
}
