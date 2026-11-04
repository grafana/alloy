package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestRunUpdateWrapsLongReasons(t *testing.T) {
	long := strings.Repeat("This reason is far too long to fit on one line. ", 4)
	handWrapped := "First line.\nSecond line."
	initial := `components:
  - name: a.comp
    stability: experimental
    reason: "` + long + `"
    expires: 2026-10-01
  - name: b.comp
    stability: experimental
    reason: |-
      ` + strings.ReplaceAll(handWrapped, "\n", "\n      ") + `
    expires: 2026-10-01
features:
  - name: short feature
    stability: experimental
    reason: "Short reason."
    expires: 2026-10-01
`
	path := filepath.Join(t.TempDir(), "stability.yaml")
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	comps := []registeredComponent{
		{Name: "a.comp", Stability: "experimental"},
		{Name: "b.comp", Stability: "experimental"},
	}
	expires := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := runUpdate(path, comps, expires, &bytes.Buffer{}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if len(line) > 80 {
			t.Errorf("line is longer than 80 columns: %q", line)
		}
	}
	if !strings.Contains(string(data), "reason: |-") {
		t.Errorf("long reason should be a literal block string:\n%s", data)
	}

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	got := byName(cfg.Components)
	// Wrapping changes only the whitespace between words.
	if a := got["a.comp"].Reason; strings.Join(strings.Fields(a), " ") != strings.TrimSpace(long) {
		t.Errorf("a.comp reason words changed: %q", a)
	}
	if b := got["b.comp"].Reason; b != handWrapped {
		t.Errorf("b.comp reason = %q, want hand-made line breaks kept", b)
	}
	if f := cfg.Features[0].Reason; f != "Short reason." {
		t.Errorf("short reason = %q, want it unchanged", f)
	}

	// Running again changes nothing.
	if err := runUpdate(path, comps, expires, &bytes.Buffer{}); err != nil {
		t.Fatalf("runUpdate second run: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Errorf("update is not idempotent:\nbefore:\n%s\nafter:\n%s", data, after)
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"", 10, ""},
		{"one two three", 20, "one two three"},
		{"one two three", 7, "one two\nthree"},
		{"  extra   spaces\nand\nbreaks ", 40, "extra spaces and breaks"},
		{"short averyveryverylongword end", 8, "short\naveryveryverylongword\nend"},
	}
	for _, tt := range tests {
		if got := wrapText(tt.in, tt.width); got != tt.want {
			t.Errorf("wrapText(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
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
