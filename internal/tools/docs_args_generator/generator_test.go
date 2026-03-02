package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunWithRealSchemaFile(t *testing.T) {
	// Test with the test1 schema which is based on the real loki.source.api schema
	testDataSchemaPath := filepath.Join("testdata", "test1", "metadata.yml")

	// Create a temporary directory for test output
	outputDir := t.TempDir()
	t.Logf("Output directory: %s", outputDir)

	// Run the function with the schema in the output directory
	require.NoError(t, generate(testDataSchemaPath, outputDir))

	// Compare the actual output with the expected output
	expectedOutputDir := filepath.Join("testdata", "test1", "expected_output")
	compareDirectories(t, expectedOutputDir, outputDir)
}

// compareDirectories compares all files in the expected directory with the actual directory
func compareDirectories(t *testing.T, expectedDir, actualDir string) {
	t.Helper()

	// Read the expected directory
	expectedFiles, err := os.ReadDir(expectedDir)
	require.NoError(t, err, "Failed to read expected output directory")

	// Ensure we have some expected files
	require.NotEmpty(t, expectedFiles, "Expected output directory is empty")

	// Read the actual directory
	actualFiles, err := os.ReadDir(actualDir)
	require.NoError(t, err, "Failed to read actual output directory")

	// Create maps for easier comparison
	expectedFileMap := make(map[string]bool)
	actualFileMap := make(map[string]bool)

	for _, file := range expectedFiles {
		if !file.IsDir() {
			expectedFileMap[file.Name()] = true
		}
	}

	for _, file := range actualFiles {
		if !file.IsDir() {
			actualFileMap[file.Name()] = true
		}
	}

	// Check that all expected files exist in actual output
	for expectedFile := range expectedFileMap {
		require.True(t, actualFileMap[expectedFile], "Expected file %s not found in actual output", expectedFile)

		// Compare file contents
		expectedPath := filepath.Join(expectedDir, expectedFile)
		actualPath := filepath.Join(actualDir, expectedFile)

		expectedContent, err := os.ReadFile(expectedPath)
		require.NoError(t, err, "Failed to read expected file %s", expectedFile)

		actualContent, err := os.ReadFile(actualPath)
		require.NoError(t, err, "Failed to read actual file %s", expectedFile)

		require.Equal(t, string(expectedContent), string(actualContent),
			"Content mismatch in file %s", expectedFile)
	}

	// Check that no unexpected files were generated
	for actualFile := range actualFileMap {
		require.True(t, expectedFileMap[actualFile],
			"Unexpected file %s found in actual output", actualFile)
	}

	t.Logf("Successfully compared %d files", len(expectedFileMap))
}
