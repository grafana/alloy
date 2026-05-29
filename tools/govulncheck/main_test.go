package govulncheck

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseSymbolFindings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "symbol findings only, ignoring package and module sections",
			in: `
=== Symbol Results ===

Vulnerability #1: GO-2026-5013
    Some title
  More info: https://example/GO-2026-5013

Vulnerability #2: GO-2026-5018
    Other title

=== Package Results ===

Vulnerability #1: GO-2026-9991
    package-only finding (not reachable)

=== Module Results ===

Vulnerability #1: GO-2026-9992
    module-only finding (not reachable)
`,
			want: []string{"GO-2026-5013", "GO-2026-5018"},
		},
		{
			name: "no findings",
			in:   "=== Symbol Results ===\n\nNo vulnerabilities found.\n",
			want: nil,
		},
		{
			name: "empty output",
			in:   "",
			want: nil,
		},
		{
			name: "accepts non-go advisory ids",
			in: `
=== Symbol Results ===

Vulnerability #1: CVE-2026-5013
Vulnerability #2: GHSA-abcd-efgh-ijkl
`,
			want: []string{"CVE-2026-5013", "GHSA-abcd-efgh-ijkl"},
		},
		{
			name: "ignores findings when symbol section is absent",
			in: `
=== Package Results ===

Vulnerability #1: GO-2026-9991
`,
			want: nil,
		},
		{
			name: "ignores malformed vulnerability header lines",
			in: `
=== Symbol Results ===

Vulnerability #1: GO-2026-5013 extra-text
Vulnerability #2 GO-2026-5014
Vulnerability #3: GO-2026-5015
`,
			want: []string{"GO-2026-5015"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSymbolFindings(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
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

func TestAdvisoryURL(t *testing.T) {
	if got, want := advisoryURL("GO-2026-1234"), "https://pkg.go.dev/vuln/GO-2026-1234"; got != want {
		t.Errorf("advisoryURL(go id) = %q, want %q", got, want)
	}
	if got := advisoryURL("CVE-2026-1234"); got != "" {
		t.Errorf("advisoryURL(non-go id) = %q, want empty", got)
	}
}

func TestResolveConfigPath(t *testing.T) {
	root := t.TempDir()
	rel := ".govulncheck.yaml"
	if got, want := resolveConfigPath(root, rel), filepath.Join(root, rel); got != want {
		t.Errorf("resolveConfigPath(relative) = %q, want %q", got, want)
	}

	abs := filepath.Join(root, "config", "custom.yaml")
	if got := resolveConfigPath(root, abs); got != abs {
		t.Errorf("resolveConfigPath(absolute) = %q, want %q", got, abs)
	}
}
