package main

import (
	"reflect"
	"testing"
	"time"
)

// Mini-fixture mirroring real govulncheck text output. Two symbol findings
// (reachable), one package finding (importable but uncalled), one module
// finding (declared in go.mod but uncalled). Only the symbol IDs should
// come back from parseSymbolFindings.
const fixture = `
=== Symbol Results ===

Vulnerability #1: GO-2026-5013
    Some title
  More info: https://example/GO-2026-5013

Vulnerability #2: GO-2026-5018
    Other title
  More info: https://example/GO-2026-5018

=== Package Results ===

Vulnerability #1: GO-2026-9991
    package-only finding (not reachable)

=== Module Results ===

Vulnerability #1: GO-2026-9992
    module-only finding (not reachable)

Your code is affected by 2 vulnerabilities from 1 module.
`

func TestParseSymbolFindings_OnlyReturnsSymbolSectionIDs(t *testing.T) {
	got := parseSymbolFindings(fixture)
	want := []string{"GO-2026-5013", "GO-2026-5018"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSymbolFindings_NoFindings(t *testing.T) {
	out := `=== Symbol Results ===

No vulnerabilities found.
`
	got := parseSymbolFindings(out)
	if len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
}

func TestParseSymbolFindings_EmptyOutput(t *testing.T) {
	got := parseSymbolFindings("")
	if len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
}

func TestClassify_SplitsByIgnoreList(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cfg := &Config{Ignore: []IgnoreEntry{
		{ID: "GO-2026-IGN", Reason: "x"},
		{ID: "GO-2026-EXPIRED", Reason: "y", Expires: now.AddDate(0, -1, 0)},
	}}
	actionable, ignored := classify(
		[]string{"GO-2026-IGN", "GO-2026-ACT", "GO-2026-EXPIRED"},
		cfg, now,
	)
	if !reflect.DeepEqual(actionable, []string{"GO-2026-ACT", "GO-2026-EXPIRED"}) {
		t.Errorf("actionable = %v", actionable)
	}
	if !reflect.DeepEqual(ignored, []string{"GO-2026-IGN"}) {
		t.Errorf("ignored = %v", ignored)
	}
}

func TestDedup(t *testing.T) {
	got := dedup([]string{"GO-2026-2", "GO-2026-1", "GO-2026-2", "GO-2026-1"})
	want := []string{"GO-2026-1", "GO-2026-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
