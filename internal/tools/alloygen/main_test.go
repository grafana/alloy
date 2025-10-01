package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWithRealSchemaFile(t *testing.T) {
	// Test with the test1 schema which is based on the real loki.source.api schema
	testDataSchemaPath := filepath.Join("testdata", "test1", "schema.yml")
	outputDir := filepath.Join("testdata", "test1", "output")

	// Check if the testdata schema file exists
	if _, err := os.Stat(testDataSchemaPath); os.IsNotExist(err) {
		t.Skip("Testdata schema file not found, skipping test")
	}

	// Clean up output directory before test
	if err := os.RemoveAll(outputDir); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to clean output directory: %v", err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// Copy the schema file to the output directory so the run function can find it
	schemaContent, err := os.ReadFile(testDataSchemaPath)
	if err != nil {
		t.Fatalf("Failed to read testdata schema: %v", err)
	}

	outputSchemaPath := filepath.Join(outputDir, "schema.yml")
	if err := os.WriteFile(outputSchemaPath, schemaContent, 0644); err != nil {
		t.Fatalf("Failed to write schema to output directory: %v", err)
	}

	// Run the function with the schema in the output directory
	err = runOnce(outputSchemaPath)
	if err != nil {
		t.Errorf("Unexpected error with testdata schema: %v", err)
		return
	}

	// Verify output file was created in the correct location
	argumentsPath := filepath.Join(outputDir, "doc_gen", "arguments.md")
	content, err := os.ReadFile(argumentsPath)
	if err != nil {
		t.Errorf("Failed to read generated arguments.md: %v", err)
		return
	}

	// Basic validation - should contain table headers and some content
	contentStr := string(content)
	if !strings.Contains(contentStr, "| Name") {
		t.Errorf("Generated content should contain table headers")
	}
	if !strings.Contains(contentStr, "forward_to") {
		t.Errorf("Generated content should contain forward_to field from schema")
	}
	if !strings.Contains(contentStr, "[]loki.LogsReceiver") {
		t.Errorf("Generated content should contain type override for forward_to")
	}

	t.Logf("Generated content:\n%s", contentStr)
}
