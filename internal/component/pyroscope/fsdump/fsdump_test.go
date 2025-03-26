package fsdump

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestFSDump_Create(t *testing.T) {
	tempDir := t.TempDir()

	// Create component
	comp, err := New(testOptions(t), Arguments{
		TargetDirectory: tempDir,
		MaxSizeBytes:    1024 * 1024,
	})
	require.NoError(t, err)
	require.NotNil(t, comp)

	// Verify directory was created
	dirInfo, err := os.Stat(tempDir)
	require.NoError(t, err)
	require.True(t, dirInfo.IsDir())
}

func TestFSDump_WriteProfile(t *testing.T) {
	tempDir := t.TempDir()

	// Create component
	comp, err := New(testOptions(t), Arguments{
		TargetDirectory: tempDir,
		MaxSizeBytes:    1024 * 1024,
		ExternalLabels: map[string]string{
			"env": "test",
		},
	})
	require.NoError(t, err)

	// Create test profile
	testData := []byte("test profile data")
	testLabels := labels.FromStrings("service_name", "test-service", "__name__", "cpu.profile")

	// Write profile
	err = comp.exporter.Appender().Append(context.Background(), testLabels, []*pyroscope.RawSample{
		{RawProfile: testData},
	})
	require.NoError(t, err)

	// Verify a file was created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.True(t, len(files[0].Name()) > 0)

	// Verify file contents
	fileContent, err := os.ReadFile(filepath.Join(tempDir, files[0].Name()))
	require.NoError(t, err)

	// Check that profile data is in the file
	require.Contains(t, string(fileContent), "test profile data")

	// Check that labels are in the file
	require.Contains(t, string(fileContent), "service_name=\"test-service\"")
	require.Contains(t, string(fileContent), "__name__=\"cpu.profile\"")

	// Check that external label was added
	require.Contains(t, string(fileContent), "env=\"test\"")
}

func TestFSDump_AppendIngest(t *testing.T) {
	tempDir := t.TempDir()

	// Create component
	comp, err := New(testOptions(t), Arguments{
		TargetDirectory: tempDir,
		MaxSizeBytes:    1024 * 1024,
	})
	require.NoError(t, err)

	// Create test profile using AppendIngest
	testData := []byte("test ingest profile data")
	testLabels := labels.FromStrings("service_name", "test-service", "__name__", "cpu.profile")

	// Write profile
	err = comp.exporter.Appender().AppendIngest(context.Background(), &pyroscope.IncomingProfile{
		RawBody: testData,
		Labels:  testLabels,
	})
	require.NoError(t, err)

	// Verify a file was created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	// Verify file contents
	fileContent, err := os.ReadFile(filepath.Join(tempDir, files[0].Name()))
	require.NoError(t, err)

	// Check that profile data is in the file
	require.Contains(t, string(fileContent), "test ingest profile data")

	// Check that labels are in the file
	require.Contains(t, string(fileContent), "service_name=\"test-service\"")
	require.Contains(t, string(fileContent), "__name__=\"cpu.profile\"")
}

func TestFSDump_RelabelDrop(t *testing.T) {
	tempDir := t.TempDir()

	// Create component with relabeling rule to drop profiles
	comp, err := New(testOptions(t), Arguments{
		TargetDirectory: tempDir,
		MaxSizeBytes:    1024 * 1024,
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: []string{"service_name"},
				Regex:        relabel.Regexp{Regexp: regexp.MustCompile("drop-me")},
				Action:       "drop",
			},
		},
	})
	require.NoError(t, err)

	// Create test profile that should be dropped
	testData := []byte("should be dropped")
	testLabels := labels.FromStrings("service_name", "drop-me", "__name__", "cpu.profile")

	// Write profile
	err = comp.exporter.Appender().Append(context.Background(), testLabels, []*pyroscope.RawSample{
		{RawProfile: testData},
	})
	require.NoError(t, err)

	// Verify no files were created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 0)

	// Now write a profile that shouldn't be dropped
	testData = []byte("should be kept")
	testLabels = labels.FromStrings("service_name", "keep-me", "__name__", "cpu.profile")

	// Write profile
	err = comp.exporter.Appender().Append(context.Background(), testLabels, []*pyroscope.RawSample{
		{RawProfile: testData},
	})
	require.NoError(t, err)

	// Verify a file was created
	files, err = os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestFSDump_DirectorySizeLimit(t *testing.T) {
	tempDir := t.TempDir()

	// Small size limit to force cleanup
	const sizeLimit int64 = 1000

	// Create component with small size limit
	comp, err := New(testOptions(t), Arguments{
		TargetDirectory: tempDir,
		MaxSizeBytes:    sizeLimit,
	})
	require.NoError(t, err)

	// Write multiple files to exceed the limit
	for i := 0; i < 5; i++ {
		// Each file will be about 300 bytes (200 bytes data + labels and delimiters)
		testData := make([]byte, 200)
		for j := range testData {
			testData[j] = byte(i)
		}

		testLabels := labels.FromStrings("service_name", "test-service", "__name__", "cpu.profile", "index", string(rune('A'+i)))

		// Write profile
		err = comp.exporter.Appender().Append(context.Background(), testLabels, []*pyroscope.RawSample{
			{RawProfile: testData},
		})
		require.NoError(t, err)

		// Sleep to ensure files have different modification times
		time.Sleep(100 * time.Millisecond)
	}

	// Trigger cleanup
	comp.cleanupOldProfiles()

	// Check that some files were removed
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)

	// With a 1000 byte limit and ~300 byte files, we expect to have about 3 files left
	// But this can vary due to label size differences, etc., so we just verify it's less than 5
	require.Less(t, len(files), 5)

	// Total directory size should be under or close to the limit
	totalSize := int64(0)
	for _, file := range files {
		info, err := file.Info()
		require.NoError(t, err)
		totalSize += info.Size()
	}

	// Allow a slight margin for the size limit since cleanup is approximate
	require.LessOrEqual(t, totalSize, sizeLimit*2)
}

// Test utility functions

func testOptions(t *testing.T) component.Options {
	return component.Options{
		ID:         "test_fsdump",
		Logger:     util.TestLogger(t),
		Registerer: prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {
			// no-op for tests
		},
	}
}
