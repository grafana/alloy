package main

import (
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 1, 0)
	past := now.AddDate(0, -1, 0)

	comps := []registeredComponent{
		{Name: "ga.comp", Stability: "generally-available"},
		{Name: "exp.tracked", Stability: "experimental"},
		{Name: "exp.untracked", Stability: "experimental"},
		{Name: "preview.expired", Stability: "public-preview"},
		{Name: "exp.mismatch", Stability: "experimental"},
		{Name: "community.comp", Community: true},
		{Name: "now.ga", Stability: "generally-available"},
	}

	cfg := &Config{Components: []Entry{
		{Name: "exp.tracked", Stability: "experimental", Reason: "ok", Expires: future},
		{Name: "preview.expired", Stability: "public-preview", Reason: "ok", Expires: past},
		{Name: "exp.mismatch", Stability: "public-preview", Reason: "ok", Expires: future},
		{Name: "community.comp", Stability: "experimental", Reason: "ok", Expires: future},
		{Name: "now.ga", Stability: "experimental", Reason: "ok", Expires: future},
		{Name: "gone.comp", Stability: "experimental", Reason: "ok", Expires: future},
	}}

	findings := classify(comps, cfg, now)

	// Index findings by name+kind for assertions.
	got := make(map[string]findingKind)
	for _, f := range findings {
		got[f.name] = f.kind
	}

	want := map[string]findingKind{
		"exp.untracked":   missingEntry,
		"preview.expired": expired,
		"exp.mismatch":    levelMismatch,
		"community.comp":  staleCommunity,
		"now.ga":          staleGA,
		"gone.comp":       staleMissing,
	}

	if len(findings) != len(want) {
		t.Fatalf("got %d findings, want %d: %+v", len(findings), len(want), findings)
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("%s: got kind %d, want %d", name, got[name], kind)
		}
	}

	// A valid, current, matching entry must produce no finding.
	if _, ok := got["exp.tracked"]; ok {
		t.Errorf("exp.tracked should not be flagged")
	}
	// GA components without entries must not be flagged.
	if _, ok := got["ga.comp"]; ok {
		t.Errorf("ga.comp should not be flagged")
	}
}
