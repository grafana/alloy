package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseConfig_Valid(t *testing.T) {
	yaml := `
components:
  - name: loki.source.foo
    stability: experimental
    reason: "waiting on upstream API to settle"
    expires: 2026-10-01
  - name: prometheus.write.bar
    stability: public-preview
    reason: "needs load testing before GA"
    expires: 2027-01-01
features:
  - name: foreach block
    stability: experimental
    reason: "iteration semantics still changing"
    expires: 2026-10-01
`
	cfg, err := parseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if got, want := len(cfg.Components), 2; got != want {
		t.Fatalf("component count = %d, want %d", got, want)
	}
	if got, want := len(cfg.Features), 1; got != want {
		t.Fatalf("feature count = %d, want %d", got, want)
	}
	wantExp := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if !cfg.Components[0].Expires.Equal(wantExp) {
		t.Errorf("Expires[0] = %v, want %v", cfg.Components[0].Expires, wantExp)
	}
}

func TestParseConfig_Errors(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantSubstr string
	}{
		{
			name:       "missing name",
			yaml:       "components:\n  - stability: experimental\n    reason: x\n    expires: 2026-10-01\n",
			wantSubstr: "name is required",
		},
		{
			name:       "duplicate name",
			yaml:       "components:\n  - name: a.b\n    stability: experimental\n    reason: x\n    expires: 2026-10-01\n  - name: a.b\n    stability: experimental\n    reason: y\n    expires: 2026-10-01\n",
			wantSubstr: "duplicate name",
		},
		{
			name:       "invalid stability",
			yaml:       "components:\n  - name: a.b\n    stability: generally-available\n    reason: x\n    expires: 2026-10-01\n",
			wantSubstr: "stability must be one of",
		},
		{
			name:       "missing reason",
			yaml:       "components:\n  - name: a.b\n    stability: experimental\n    expires: 2026-10-01\n",
			wantSubstr: "reason is required",
		},
		{
			name:       "placeholder reason rejected",
			yaml:       "components:\n  - name: a.b\n    stability: experimental\n    reason: TODO\n    expires: 2026-10-01\n",
			wantSubstr: "placeholder",
		},
		{
			name:       "missing expires",
			yaml:       "components:\n  - name: a.b\n    stability: experimental\n    reason: x\n",
			wantSubstr: "expires is required",
		},
		{
			name:       "entries not sorted by name",
			yaml:       "components:\n  - name: b.comp\n    stability: experimental\n    reason: x\n    expires: 2026-10-01\n  - name: a.comp\n    stability: experimental\n    reason: y\n    expires: 2026-10-01\n",
			wantSubstr: "must be sorted by name",
		},
		{
			name:       "unknown field rejected",
			yaml:       "components:\n  - name: a.b\n    stability: experimental\n    reason: x\n    expires: 2026-10-01\n    severity: high\n",
			wantSubstr: "field severity not found",
		},
		{
			name:       "feature missing reason",
			yaml:       "features:\n  - name: foreach block\n    stability: experimental\n    expires: 2026-10-01\n",
			wantSubstr: "features[0] (foreach block): reason is required",
		},
		{
			name:       "features not sorted by name",
			yaml:       "features:\n  - name: foreach block\n    stability: experimental\n    reason: x\n    expires: 2026-10-01\n  - name: Windows process priority\n    stability: public-preview\n    reason: y\n    expires: 2026-10-01\n",
			wantSubstr: "features[1] (Windows process priority): entries must be sorted by name",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseConfig([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestLoadConfig_MissingFileIsEmpty(t *testing.T) {
	cfg, err := loadConfig("./does-not-exist.yaml")
	if err != nil {
		t.Fatalf("missing file should be empty config, got error: %v", err)
	}
	if len(cfg.Components) != 0 {
		t.Errorf("empty config expected, got %d entries", len(cfg.Components))
	}
}

func TestParseConfig_EmptyAndCommentOnlyFilesAreValid(t *testing.T) {
	for _, in := range []string{"", "# only a comment\n"} {
		cfg, err := parseConfig([]byte(in))
		if err != nil {
			t.Errorf("parseConfig(%q): unexpected error %v", in, err)
			continue
		}
		if len(cfg.Components) != 0 {
			t.Errorf("parseConfig(%q): want empty, got %d entries", in, len(cfg.Components))
		}
	}
}
