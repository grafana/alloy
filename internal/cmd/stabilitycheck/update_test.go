package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRunUpdate(t *testing.T) {
	expires := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)

	// Start unsorted, missing one non-GA component, with a stale entry and an
	// existing entry whose reason must be preserved.
	initial := `# managed header
components:
  - name: z.comp
    stability: experimental
    reason: "real reason to keep"
    expires: 2026-10-01
  - name: a.comp
    stability: experimental
    reason: TODO
    expires: 2026-10-01
  - name: stale.comp
    stability: experimental
    reason: "was tracked"
    expires: 2026-10-01
features:
  - name: zeta feature
    stability: experimental
    reason: TODO
    expires: 2026-10-01
  - name: alpha feature
    stability: public-preview
    reason: TODO
    expires: 2026-10-01
`
	dir := t.TempDir()
	path := filepath.Join(dir, "stability.yaml")
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	comps := []registeredComponent{
		{Name: "a.comp", Stability: "experimental"},
		{Name: "z.comp", Stability: "experimental"},
		{Name: "m.comp", Stability: "public-preview"}, // new, must be added
		{Name: "ga.comp", Stability: "generally-available"},
		{Name: "community.comp", Community: true},
		// stale.comp is absent from the registry.
	}

	var out bytes.Buffer
	if err := runUpdate(path, comps, expires, &out); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	// Components sorted by name, with the new entry inserted.
	wantComps := []string{"a.comp", "m.comp", "stale.comp", "z.comp"}
	if got := names(cfg.Components); !equal(got, wantComps) {
		t.Errorf("components = %v, want %v", got, wantComps)
	}

	// The new entry has the detected stability, a TODO reason, and the given expiry.
	m := byName(cfg.Components)["m.comp"]
	if m.Stability != "public-preview" || m.Reason != "TODO" || !m.Expires.Equal(expires) {
		t.Errorf("m.comp = %+v, want stability=public-preview reason=TODO expires=%v", m, expires)
	}

	// An existing real reason is preserved untouched.
	if got := byName(cfg.Components)["z.comp"].Reason; got != "real reason to keep" {
		t.Errorf("z.comp reason = %q, want it preserved", got)
	}

	// The stale entry is kept, not removed.
	if _, ok := byName(cfg.Components)["stale.comp"]; !ok {
		t.Errorf("stale.comp should be kept in place")
	}

	// Features are sorted but never added to.
	wantFeatures := []string{"alpha feature", "zeta feature"}
	if got := names(cfg.Features); !equal(got, wantFeatures) {
		t.Errorf("features = %v, want %v", got, wantFeatures)
	}

	// The report mentions the added component and the stale one.
	report := out.String()
	if !bytes.Contains([]byte(report), []byte("m.comp")) {
		t.Errorf("report should mention added m.comp:\n%s", report)
	}
	if !bytes.Contains([]byte(report), []byte("stale.comp")) {
		t.Errorf("report should mention stale.comp:\n%s", report)
	}

	// The managed header comment survives the rewrite.
	if !bytes.Contains(data, []byte("# managed header")) {
		t.Errorf("header comment lost:\n%s", data)
	}

	// Running again changes nothing.
	before := data
	var out2 bytes.Buffer
	if err := runUpdate(path, comps, expires, &out2); err != nil {
		t.Fatalf("runUpdate second run: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Errorf("update is not idempotent:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func names(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}
	return out
}

func byName(entries []Entry) map[string]Entry {
	out := make(map[string]Entry, len(entries))
	for _, e := range entries {
		out[e.Name] = e
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
