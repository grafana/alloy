package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMultipleTlsBlocks reproduces the bug where multiple blocks share the same
// sub-block name (e.g. both "http" and "grpc" have a "tls" child block), causing
// the [tls]: #tls link definition to be emitted once per occurrence instead of
// exactly once.
func TestMultipleTlsBlocks(t *testing.T) {
	testDataSchemaPath := filepath.Join("testdata", "test2", "metadata.yml")

	outputDir := t.TempDir()
	t.Logf("Output directory: %s", outputDir)

	require.NoError(t, generate(testDataSchemaPath, outputDir, ""))

	expectedOutputDir := filepath.Join("testdata", "test2", "expected_output")
	compareDirectories(t, expectedOutputDir, outputDir)
}

// TestDuplicateBlockNamesWithDifferentContent reproduces the bug where multiple
// blocks share the same sub-block name but define it with different properties
// (e.g. http > tls has cert_file while otlp > tls has key_pem). The [tls]: #tls
// link definition is still emitted once per occurrence. Unlike TestMultipleTlsBlocks,
// this test only compares __blocks.md because the argument table files for the
// differently-configured tls blocks are non-deterministic (the last-written version
// wins, and iteration order over Go maps is random).
func TestDuplicateBlockNamesWithDifferentContent(t *testing.T) {
	testDataSchemaPath := filepath.Join("testdata", "test3", "metadata.yml")

	outputDir := t.TempDir()
	t.Logf("Output directory: %s", outputDir)

	require.NoError(t, generate(testDataSchemaPath, outputDir, ""))

	actualContent, err := os.ReadFile(filepath.Join(outputDir, "__blocks.md"))
	require.NoError(t, err, "Failed to read actual __blocks.md")

	expectedContent, err := os.ReadFile(filepath.Join("testdata", "test3", "expected_output", "__blocks.md"))
	require.NoError(t, err, "Failed to read expected __blocks.md")

	require.Equal(t, string(expectedContent), string(actualContent), "Content mismatch in __blocks.md")
}

// TestImportedBlocksWithSameSubBlockName is similar to
// TestDuplicateBlockNamesWithDifferentContent but the blocks are defined in a
// subschema file (imported via $ref / $defs) rather than inline. This exercises
// the schema-ID-based link ID path: because the subschema carries an "id" field,
// each block's link ID is prefixed with the schema slug (e.g. "server--grpc--tls")
// rather than just the parent hierarchy.
//
// Only __blocks.md is compared because the two differently-configured tls blocks
// produce argument table files non-deterministically (last-writer-wins on the
// shared "tls" key).
func TestImportedBlocksWithSameSubBlockName(t *testing.T) {
	testDataSchemaPath := filepath.Join("testdata", "test4", "metadata.yml")

	outputDir := t.TempDir()
	t.Logf("Output directory: %s", outputDir)

	require.NoError(t, generate(testDataSchemaPath, outputDir, ""))

	actualContent, err := os.ReadFile(filepath.Join(outputDir, "__blocks.md"))
	require.NoError(t, err, "Failed to read actual __blocks.md")

	expectedContent, err := os.ReadFile(filepath.Join("testdata", "test4", "expected_output", "__blocks.md"))
	require.NoError(t, err, "Failed to read expected __blocks.md")

	require.Equal(t, string(expectedContent), string(actualContent), "Content mismatch in __blocks.md")
}

func TestRunWithRealSchemaFile(t *testing.T) {
	// Test with the test1 schema which is based on the real loki.source.api schema
	testDataSchemaPath := filepath.Join("testdata", "test1", "metadata.yml")

	// Create a temporary directory for test output
	outputDir := t.TempDir()
	t.Logf("Output directory: %s", outputDir)

	// Run the function with the schema in the output directory
	require.NoError(t, generate(testDataSchemaPath, outputDir, ""))

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
