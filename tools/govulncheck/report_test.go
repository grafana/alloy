package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestReport_ActionableVsIgnored(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cfg := &Config{Ignore: []IgnoreEntry{
		{ID: "GO-2026-IGN", Reason: "client-only path"},
		{ID: "GO-2026-EXPIRED", Reason: "expired", Expires: now.AddDate(0, -1, 0)},
	}}
	r := newReport(cfg, now)

	r.add("./modA", &scanResult{Vulns: []vulnerability{
		{ID: "GO-2026-IGN", Summary: "Should be ignored"},
		{ID: "GO-2026-ACT", Summary: "Should remain actionable", FixedVersion: "v2.0.0", Module: "example.com/x"},
	}})
	r.add("./modB", &scanResult{Vulns: []vulnerability{
		{ID: "GO-2026-EXPIRED", Summary: "expiry passed"},
	}})

	if !r.hasActionable() {
		t.Fatal("expected actionable findings (one direct + one expired-ignore)")
	}

	var out bytes.Buffer
	r.print(&out)
	s := out.String()

	wantContains := []string{
		"==> ./modA",
		"[FAIL] GO-2026-ACT",
		"fix:     upgrade to v2.0.0",
		"[IGN]  GO-2026-IGN",
		"client-only path",
		"==> ./modB",
		"[FAIL] GO-2026-EXPIRED",
		"Summary: 2 actionable, 1 ignored",
	}
	for _, w := range wantContains {
		if !strings.Contains(s, w) {
			t.Errorf("output missing %q. Full output:\n%s", w, s)
		}
	}
}

func TestReport_AllClean(t *testing.T) {
	r := newReport(&Config{}, time.Now())
	r.add("./modA", &scanResult{})
	if r.hasActionable() {
		t.Fatal("expected no actionable findings")
	}
	var out bytes.Buffer
	r.print(&out)
	if !strings.Contains(out.String(), "no reachable vulnerabilities") {
		t.Errorf("expected clean-module marker, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "OK — no actionable") {
		t.Errorf("expected OK summary, got:\n%s", out.String())
	}
}

func TestRenderTrace_ReversesOrder(t *testing.T) {
	// govulncheck emits frames vuln-first, entry-point-last.
	// We render entry-point first so it reads top-down like a stack.
	trace := []frame{
		{Function: "vulnFunc"},
		{Function: "middle"},
		{Function: "main"},
	}
	got := renderTrace(trace)
	want := "main → middle → vulnFunc"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
