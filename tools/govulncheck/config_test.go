package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseConfig_Valid(t *testing.T) {
	yaml := `
ignore:
  - id: GO-2026-4887
    reason: "client only, not a daemon"
    expires: 2027-01-01
  - id: GO-2026-4883
    reason: "plugin install not used"
`
	cfg, err := parseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if got, want := len(cfg.Ignore), 2; got != want {
		t.Fatalf("ignore count = %d, want %d", got, want)
	}
	wantExp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	if !cfg.Ignore[0].Expires.Equal(wantExp) {
		t.Errorf("Expires[0] = %v, want %v", cfg.Ignore[0].Expires, wantExp)
	}
	if !cfg.Ignore[1].Expires.IsZero() {
		t.Errorf("Expires[1] should be zero (no expiry), got %v", cfg.Ignore[1].Expires)
	}
}

func TestParseConfig_Errors(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantSubstr  string
	}{
		{
			name:       "invalid id format",
			yaml:       "ignore:\n  - id: CVE-2024-1234\n    reason: x\n",
			wantSubstr: "is not a valid GO-YYYY-NNNN",
		},
		{
			name:       "missing reason",
			yaml:       "ignore:\n  - id: GO-2026-1234\n",
			wantSubstr: "reason is required",
		},
		{
			name:       "duplicate id",
			yaml:       "ignore:\n  - id: GO-2026-1\n    reason: a\n  - id: GO-2026-1\n    reason: b\n",
			wantSubstr: "duplicate id",
		},
		{
			name:       "unknown field rejected",
			yaml:       "ignore:\n  - id: GO-2026-1\n    reason: a\n    severity: high\n",
			wantSubstr: "field severity not found",
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

func TestConfig_IsIgnored(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cfg := &Config{Ignore: []IgnoreEntry{
		{ID: "GO-2026-100", Reason: "current"},
		{ID: "GO-2026-200", Reason: "future", Expires: now.AddDate(0, 1, 0)},
		{ID: "GO-2026-300", Reason: "expired", Expires: now.AddDate(0, -1, 0)},
	}}

	t.Run("not in list", func(t *testing.T) {
		if e := cfg.isIgnored("GO-2026-999", now); e != nil {
			t.Errorf("got %+v, want nil", e)
		}
	})
	t.Run("no expiry", func(t *testing.T) {
		if e := cfg.isIgnored("GO-2026-100", now); e == nil {
			t.Errorf("got nil, want match")
		}
	})
	t.Run("future expiry still valid", func(t *testing.T) {
		if e := cfg.isIgnored("GO-2026-200", now); e == nil {
			t.Errorf("got nil, want match")
		}
	})
	t.Run("expired entry no longer applies", func(t *testing.T) {
		if e := cfg.isIgnored("GO-2026-300", now); e != nil {
			t.Errorf("got %+v, want nil (expired)", e)
		}
	})
}

func TestLoadConfig_MissingFileIsEmpty(t *testing.T) {
	cfg, err := loadConfig("./does-not-exist.yaml")
	if err != nil {
		t.Fatalf("missing file should be empty config, got error: %v", err)
	}
	if len(cfg.Ignore) != 0 {
		t.Errorf("empty config expected, got %d entries", len(cfg.Ignore))
	}
}
