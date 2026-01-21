package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/replace-generator/cmd"
)

type testCase struct {
	name        string
	testdataDir string
}

func TestE2EMod(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	command := cmd.NewRootCommand()

	var testCases = []testCase{
		{
			name:        "Basic",
			testdataDir: "basic-mod",
		},
		{
			name:        "UpdateExisting",
			testdataDir: "update-existing-mod",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testdataDir := filepath.Join(currentDir, "testdata", tc.testdataDir)
			goModPath := filepath.Join(testdataDir, "go.mod")
			expectedPath := filepath.Join(testdataDir, "go.mod.expected")
			dependencyYaml := filepath.Join("testdata", tc.testdataDir, "dependency-replacements-test.yaml")
			projectRoot := filepath.Join("testdata", tc.testdataDir)

			originalGoMod, err := os.ReadFile(goModPath)
			if err != nil {
				t.Fatalf("Failed to read original go.mod: %v", err)
			}

			// Restore the original go.mod after the test
			defer func() {
				if err := os.WriteFile(goModPath, originalGoMod, 0644); err != nil {
					t.Errorf("Failed to restore original go.mod: %v", err)
				}
			}()

			command.SetArgs([]string{"generate", "--dependency-yaml", dependencyYaml, "--project-root", projectRoot})
			err = command.Execute()
			if err != nil {
				t.Fatalf("Failed to run command: %v", err)
			}

			expectedContent, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("Failed to read expected go.mod: %v", err)
			}
			expectedGoMod := string(normalizeLineEndings(expectedContent))

			actualContent, err := os.ReadFile(goModPath)
			if err != nil {
				t.Fatalf("Failed to read actual go.mod: %v", err)
			}
			actualGoMod := string(normalizeLineEndings(actualContent))

			if actualGoMod != expectedGoMod {
				t.Errorf("go.mod content mismatch.\nExpected:\n%s\n\nActual:\n%s", expectedGoMod, actualGoMod)
			}
		})
	}
}

func TestE2EOCB(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	command := cmd.NewRootCommand()

	var testCases = []testCase{
		{
			name:        "Basic",
			testdataDir: "basic-ocb",
		},
		{
			name:        "UpdateExisting",
			testdataDir: "update-existing-ocb",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testdataDir := filepath.Join(currentDir, "testdata", tc.testdataDir)
			builderYamlPath := filepath.Join(testdataDir, "test-builder-config.yaml")
			expectedPath := filepath.Join(testdataDir, "test-builder-config-expected.yaml")
			dependencyYaml := filepath.Join("testdata", tc.testdataDir, "dependency-replacements-test.yaml")
			projectRoot := filepath.Join("testdata", tc.testdataDir)

			originalYaml, err := os.ReadFile(builderYamlPath)
			if err != nil {
				t.Fatalf("Failed to read original builder yaml: %v", err)
			}

			// Restore the original builder yaml after the test
			defer func() {
				if err := os.WriteFile(builderYamlPath, originalYaml, 0644); err != nil {
					t.Errorf("Failed to restore original builder yaml: %v", err)
				}
			}()

			command.SetArgs([]string{"generate", "--dependency-yaml", dependencyYaml, "--project-root", projectRoot})
			err = command.Execute()
			if err != nil {
				t.Fatalf("Failed to run command: %v", err)
			}

			expectedContent, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("Failed to read expected builder yaml: %v", err)
			}
			expectedYaml := string(normalizeLineEndings(expectedContent))

			actualContent, err := os.ReadFile(builderYamlPath)
			if err != nil {
				t.Fatalf("Failed to read actual builder yaml: %v", err)
			}
			actualYaml := string(normalizeLineEndings(actualContent))

			if actualYaml != expectedYaml {
				t.Errorf("builder yaml content mismatch.\nExpected:\n%s\n\nActual:\n%s", expectedYaml, actualYaml)
			}
		})
	}
}

// normalizeLineEndings will replace '\r\n' with '\n'.
func normalizeLineEndings(data []byte) []byte {
	normalized := bytes.TrimSpace(bytes.ReplaceAll(data, []byte{'\r', '\n'}, []byte{'\n'}))
	return normalized
}
