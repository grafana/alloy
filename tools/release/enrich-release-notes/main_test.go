package main

import "testing"

func TestExtractCommitSHA(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedSHA string
	}{
		{
			name:        "commit link at end of line",
			input:       "* HTTP/2 is no longer always disabled in loki.write ([#5267](https://github.com/grafana/alloy/issues/5267)) ([1c97c2d](https://github.com/grafana/alloy/commit/1c97c2d569fcda2f6761534150b063d1404dc388))",
			expectedSHA: "1c97c2d",
		},
		{
			name:        "commit link with closes reference after",
			input:       "* Invalid handling of `id` in `foreach` when using discovery components ([#5322](https://github.com/grafana/alloy/issues/5322)) ([61fe184](https://github.com/grafana/alloy/commit/61fe1845d3b109992cbb0ec99a062ac113c1a411)), closes [#5297](https://github.com/grafana/alloy/issues/5297)",
			expectedSHA: "61fe184",
		},
		{
			name:        "commit link with extra notes after",
			input:       "* Some fix ([deadbeef](https://github.com/grafana/alloy/commit/deadbeef)) - extra notes here",
			expectedSHA: "deadbeef",
		},
		{
			name:        "full 40-character SHA",
			input:       "* Fix bug ([abc1234567890def1234567890abc1234567890](https://github.com/grafana/alloy/commit/abc1234567890def1234567890abc1234567890))",
			expectedSHA: "abc1234567890def1234567890abc1234567890",
		},
		{
			name:        "no parens around link",
			input:       "* No parens [abc1234](https://github.com/grafana/alloy/commit/abc1234)",
			expectedSHA: "",
		},
		{
			name:        "just a PR reference",
			input:       "* Just a PR reference (#1234)",
			expectedSHA: "",
		},
		{
			name:        "empty line",
			input:       "",
			expectedSHA: "",
		},
		{
			name:        "line with no commit info",
			input:       "### Bug Fixes",
			expectedSHA: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sha := extractCommitSHA(tt.input)
			if sha != tt.expectedSHA {
				t.Errorf("extractCommitSHA(%q) = %q, want %q", tt.input, sha, tt.expectedSHA)
			}
		})
	}
}

func TestFormatAttribution(t *testing.T) {
	tests := []struct {
		name      string
		usernames []string
		expected  string
	}{
		{
			name:      "single user",
			usernames: []string{"alice"},
			expected:  "(@alice)",
		},
		{
			name:      "multiple users",
			usernames: []string{"alice", "bob", "charlie"},
			expected:  "(@alice, @bob, @charlie)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAttribution(tt.usernames)
			if result != tt.expected {
				t.Errorf("formatAttribution(%v) = %q, want %q", tt.usernames, result, tt.expected)
			}
		})
	}
}

func TestDeriveDocTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "standard release",
			tag:      "v1.15.2",
			expected: "v1.15",
		},
		{
			name:     "release candidate",
			tag:      "v1.2.3-rc.0",
			expected: "v1.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := deriveDocTag(tt.tag)
			if err != nil {
				t.Fatalf("deriveDocTag(%q) returned error: %v", tt.tag, err)
			}
			if result != tt.expected {
				t.Errorf("deriveDocTag(%q) = %q, want %q", tt.tag, result, tt.expected)
			}
		})
	}
}
