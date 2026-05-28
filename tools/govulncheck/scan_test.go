package main

import (
	"strings"
	"testing"
)

func TestParseScanOutput_SymbolLevelFinding(t *testing.T) {
	// A trimmed-down govulncheck -json stream containing:
	//   1. config message (ignored by wrapper)
	//   2. osv message with summary (used to enrich findings)
	//   3. a symbol-level finding (reachable, function set on trace[0])
	//   4. a module-level finding (not reachable, empty function)
	//   5. a duplicate symbol-level finding for the same OSV (should be deduped)
	stream := `
{"config":{"scanner_version":"v1.3.0","scan_level":"symbol"}}
{"osv":{"id":"GO-2026-1234","summary":"Test vuln summary"}}
{"finding":{"osv":"GO-2026-1234","fixed_version":"v1.2.3","trace":[
  {"module":"example.com/dep","package":"example.com/dep","function":"Bad","position":{"filename":"x.go","line":10}},
  {"module":"example.com/mine","package":"example.com/mine","function":"Caller","position":{"filename":"main.go","line":5}}
]}}
{"finding":{"osv":"GO-2026-9999","trace":[
  {"module":"example.com/other"}
]}}
{"finding":{"osv":"GO-2026-1234","fixed_version":"v1.2.3","trace":[
  {"module":"example.com/dep","package":"example.com/dep","function":"Bad"},
  {"module":"example.com/mine","package":"example.com/mine","function":"OtherCaller"}
]}}
`
	result, err := parseScanOutput(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("parseScanOutput: %v", err)
	}
	if got, want := len(result.Vulns), 1; got != want {
		t.Fatalf("got %d vulns, want %d (module-level should be filtered, dup should be deduped)", got, want)
	}
	v := result.Vulns[0]
	if v.ID != "GO-2026-1234" {
		t.Errorf("ID = %s, want GO-2026-1234", v.ID)
	}
	if v.Summary != "Test vuln summary" {
		t.Errorf("Summary = %q, want enriched from osv message", v.Summary)
	}
	if v.FixedVersion != "v1.2.3" {
		t.Errorf("FixedVersion = %s, want v1.2.3", v.FixedVersion)
	}
	if v.Module != "example.com/dep" {
		t.Errorf("Module = %s, want example.com/dep (taken from trace[0])", v.Module)
	}
}

func TestParseScanOutput_NoFindings(t *testing.T) {
	stream := `{"config":{"scanner_version":"v1.3.0"}}`
	result, err := parseScanOutput(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("parseScanOutput: %v", err)
	}
	if len(result.Vulns) != 0 {
		t.Errorf("want 0 vulns, got %d", len(result.Vulns))
	}
}

func TestParseScanOutput_PackageLevelNotReported(t *testing.T) {
	// A package-level finding has a trace with package set but no function.
	// govulncheck's own HasCalledStack returns false here, so we shouldn't
	// either — only symbol-level (reachable) findings should fail the build.
	stream := `{"finding":{"osv":"GO-2026-5555","trace":[
  {"module":"example.com/dep","package":"example.com/dep"}
]}}`
	result, err := parseScanOutput(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("parseScanOutput: %v", err)
	}
	if len(result.Vulns) != 0 {
		t.Errorf("want 0 vulns (package-level shouldn't be reachable), got %d", len(result.Vulns))
	}
}

func TestParseScanOutput_MalformedJSONReturnsError(t *testing.T) {
	stream := `{"finding":{"osv":"GO-1","trace":[]}}{not json`
	_, err := parseScanOutput(strings.NewReader(stream))
	if err == nil {
		t.Fatal("want error on malformed JSON, got nil")
	}
}
