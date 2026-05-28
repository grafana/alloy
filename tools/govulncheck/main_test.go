package govulncheck

import (
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
