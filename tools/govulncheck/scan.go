package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// message mirrors the top-level Message type emitted by
// golang.org/x/vuln/cmd/govulncheck in -json mode. Only the subset of fields
// the wrapper needs is decoded — unknown ones are ignored by json.Unmarshal.
//
// Schema reference:
// https://pkg.go.dev/golang.org/x/vuln/internal/govulncheck (Message + Finding + Frame)
type message struct {
	Config  *struct {
		ScannerVersion string `json:"scanner_version,omitempty"`
		ScanLevel      string `json:"scan_level,omitempty"`
	} `json:"config,omitempty"`
	OSV     *osvEntry `json:"osv,omitempty"`
	Finding *finding  `json:"finding,omitempty"`
}

type osvEntry struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

type finding struct {
	OSV          string  `json:"osv,omitempty"`
	FixedVersion string  `json:"fixed_version,omitempty"`
	Trace        []frame `json:"trace,omitempty"`
}

type frame struct {
	Module   string `json:"module"`
	Package  string `json:"package,omitempty"`
	Function string `json:"function,omitempty"`
	Position *struct {
		Filename string `json:"filename,omitempty"`
		Line     int    `json:"line,omitempty"`
		Column   int    `json:"column,omitempty"`
	} `json:"position,omitempty"`
}

// reachable reports whether this finding is symbol-level (i.e. the vulnerable
// function is in the call graph). Mirrors govulncheck's own HasCalledStack:
// https://github.com/golang/vuln/blob/v1.3.0/internal/scan/template.go#L97
func (f *finding) reachable() bool {
	return len(f.Trace) > 0 && f.Trace[0].Function != ""
}

// vulnerability is one OSV ID's worth of findings within a single module
// scan, after de-duplicating across the multiple trace messages govulncheck
// emits for the same vuln.
type vulnerability struct {
	ID           string
	Summary      string
	FixedVersion string
	Module       string // module path the vuln lives in
	// ExampleTrace is one human-readable call chain, used for the report.
	// govulncheck emits frames root-first (vulnerable symbol first, entry
	// point last), so we render them in reverse.
	ExampleTrace []frame
}

// scanResult is the de-duplicated set of reachable vulnerabilities found in a
// single module.
type scanResult struct {
	Vulns []vulnerability
}

// discoverModules returns the list of Go module directories under root,
// excluding any directory whose path contains a "testdata" segment. The
// returned paths are relative to root and sorted for deterministic output.
func discoverModules(root string) ([]string, error) {
	var modules []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		dir := filepath.Dir(path)
		for _, part := range strings.Split(filepath.ToSlash(dir), "/") {
			if part == "testdata" {
				return nil
			}
		}
		modules = append(modules, dir)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(modules)
	return modules, nil
}

// scanModule runs govulncheck against the module rooted at dir and decodes
// its JSON stream. tags is passed through as the comma-separated -tags flag.
// The returned scanResult contains only symbol-level (reachable) findings,
// de-duplicated by OSV ID with one example trace each.
func scanModule(dir, tags string) (*scanResult, error) {
	args := []string{"run", govulncheckPkg, "-json"}
	if tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, "./...")

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// govulncheck itself returns 0 in -json mode regardless of findings
		// (the wrapper decides exit code based on filter result), so any
		// non-zero exit here is a real tool failure.
		return nil, fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}

	return parseScanOutput(&stdout)
}

// parseScanOutput consumes a govulncheck -json stream and returns the
// de-duplicated reachable vulnerabilities. Split out from scanModule for
// testing against fixture JSON.
func parseScanOutput(r io.Reader) (*scanResult, error) {
	dec := json.NewDecoder(r)
	osvByID := map[string]*osvEntry{}
	vulnByID := map[string]*vulnerability{}

	for {
		var msg message
		if err := dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode govulncheck message: %w", err)
		}
		switch {
		case msg.OSV != nil:
			osvByID[msg.OSV.ID] = msg.OSV
		case msg.Finding != nil && msg.Finding.reachable():
			if _, ok := vulnByID[msg.Finding.OSV]; ok {
				continue // already captured one trace for this OSV; one example is enough
			}
			v := &vulnerability{
				ID:           msg.Finding.OSV,
				FixedVersion: msg.Finding.FixedVersion,
				ExampleTrace: msg.Finding.Trace,
			}
			if len(msg.Finding.Trace) > 0 {
				v.Module = msg.Finding.Trace[0].Module
			}
			vulnByID[msg.Finding.OSV] = v
		}
	}

	// Decorate with summaries from the OSV messages.
	for id, v := range vulnByID {
		if o, ok := osvByID[id]; ok {
			v.Summary = o.Summary
		}
	}

	ids := make([]string, 0, len(vulnByID))
	for id := range vulnByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := &scanResult{Vulns: make([]vulnerability, 0, len(ids))}
	for _, id := range ids {
		result.Vulns = append(result.Vulns, *vulnByID[id])
	}
	return result, nil
}
