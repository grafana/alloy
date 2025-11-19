// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Defines the structures used in (de)serializing the coredump test cases.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"
)

// CoredumpTestCase is the data structure generated from the core dump.
type CoredumpTestCase struct {
	CoredumpRef modulestore.ID `json:"coredump-ref"`
	Skip        string         `json:"skip,omitempty"`
	Threads     []ThreadInfo   `json:"threads"`
	Modules     []ModuleInfo   `json:"modules"`
}

// ModuleInfo stores information about a module that was loaded when the coredump was created.
type ModuleInfo struct {
	// Ref is a reference to the module's ELF binary in the module store.
	Ref modulestore.ID `json:"ref"`
	// LocalPath stores the path where the module was found when the coredump was created.
	LocalPath string `json:"local-path"`
}

// ThreadInfo describe stack state of one thread inside core dump.
type ThreadInfo struct {
	LWP    uint32   `json:"lwp"`
	Frames []string `json:"frames"`
}

// findTestCases returns a list of all test cases, optionally only for the current architecture.
func findTestCases(filterHostArch bool) ([]string, error) {
	var arch string
	if filterHostArch {
		arch = runtime.GOARCH
	} else {
		arch = "*"
	}

	pattern := fmt.Sprintf("./testdata/%s/*.json", arch)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to locate test cases: %w", err)
	}

	return matches, nil
}

// makeTestCasePath creates the relative file path for a test case given a name.
func makeTestCasePath(name string) string {
	return fmt.Sprintf("testdata/%s/%s.json", runtime.GOARCH, name)
}

// writeTestCase writes a test case to disk.
func writeTestCase(path string, c *CoredumpTestCase, allowOverwrite bool) error {
	flags := os.O_RDWR | os.O_CREATE
	if allowOverwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	jsonFile, err := os.OpenFile(path, flags, 0o666)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}

	enc := json.NewEncoder(jsonFile)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("JSON Marshall failed: %w", err)
	}

	return nil
}

// readTestCase reads a test case from the given path.
func readTestCase(path string) (*CoredumpTestCase, error) {
	test := &CoredumpTestCase{}
	if err := readJSON(path, test); err != nil {
		return nil, err
	}
	return test, nil
}

// readJSON reads a JSON file and unmarshalls it into the given object.
func readJSON(path string, to any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	return dec.Decode(to)
}
