package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPseudoVersion(t *testing.T) {
	tests := []struct {
		version string
		commit  string
	}{
		{"v0.0.0-20261016145052-44e4679ca78b", "44e4679ca78b"},
		{"v0.11.1-0.20220518131644-f9522724dab4", "f9522724dab4"},
		{"v0.18.1-grafana-r01.0.20251024135609-318b01780c89", "318b01780c89"},
		{"v1.10.2", ""},
	}
	for _, tt := range tests {
		m := pseudoVersionRe.FindStringSubmatch(tt.version)
		got := ""
		if m != nil {
			got = m[2]
		}
		if got != tt.commit {
			t.Errorf("%s: got commit %q, want %q", tt.version, got, tt.commit)
		}
	}
}

func TestReplaceComments(t *testing.T) {
	gomod := `module example

require a v1.0.0

// first line
// second line
replace a => b v1.0.1

replace c => d v2.0.0

// orphaned

replace e => f v3.0.0
`
	got, err := replaceComments(strings.NewReader(gomod))
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != "first line second line" {
		t.Errorf("a: got %q", got["a"])
	}
	if _, ok := got["c"]; ok {
		t.Errorf("c has no comment but got %q", got["c"])
	}
	if _, ok := got["e"]; ok {
		t.Errorf("e's comment is separated by a blank line but got %q", got["e"])
	}
}

func TestSemverLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v1.5.0", "v1.11.0", true},
		{"v1.13.0", "v1.13.0", false},
		{"v0.24.10", "0.24.11", true},
		{"v2.0.0", "v1.9.9", false},
		{"v1.0.0", "not-a-version", false},
	}
	for _, tt := range tests {
		if got := semverLess(tt.a, tt.b); got != tt.want {
			t.Errorf("semverLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestGate(t *testing.T) {
	tests := []struct {
		name  string
		e     exporter
		audit bool
	}{
		{
			name:  "fork always audits",
			e:     exporter{ForkRepo: "grafana/x", Latest: &release{Tag: "v1.0.0"}, Pin: pin{Required: "v1.0.0"}},
			audit: true,
		},
		{
			name:  "tagged and current",
			e:     exporter{Latest: &release{Tag: "v1.2.0"}, Pin: pin{Required: "v1.2.0"}, Drift: &drift{UpstreamOnly: 40}},
			audit: false,
		},
		{
			name:  "tagged and behind",
			e:     exporter{Latest: &release{Tag: "v1.3.0"}, Pin: pin{Required: "v1.2.0"}},
			audit: true,
		},
		{
			name:  "commit older than latest release",
			e:     exporter{Latest: &release{Tag: "v1.0.0", Published: "2026-06-01"}, PinCommit: "abc", PinDate: "2022-05-18"},
			audit: true,
		},
		{
			name:  "commit behind default branch with no releases",
			e:     exporter{PinCommit: "abc", PinDate: "2026-04-28", Drift: &drift{UpstreamOnly: 3}},
			audit: true,
		},
		{
			name:  "tag behind default branch with no releases",
			e:     exporter{Pin: pin{Required: "v0.3.0"}, Drift: &drift{UpstreamOnly: 6}},
			audit: true,
		},
		{
			name:  "commit at default branch head",
			e:     exporter{PinCommit: "abc", PinDate: "2026-10-07", Drift: &drift{UpstreamOnly: 0}},
			audit: false,
		},
		{
			name:  "failed lookups are not silently skipped",
			e:     exporter{Pin: pin{Required: "v1.0.0"}, Errors: []string{"boom"}},
			audit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, reason := gate(&tt.e); got != tt.audit {
				t.Errorf("gate = %v (%s), want %v", got, reason, tt.audit)
			}
		})
	}
}

func TestDiscoverComponentsReportsUnmapped(t *testing.T) {
	repo := t.TempDir()
	for _, d := range []string{"kafka", "common", "brand_new"} {
		if err := os.MkdirAll(filepath.Join(repo, componentsDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	components, problems, err := discoverComponents(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(components) != 1 || components[0] != "kafka" {
		t.Errorf("components = %v, want [kafka]", components)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "brand_new") {
		t.Errorf("problems = %v, want one mentioning brand_new", problems)
	}
}
