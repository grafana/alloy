package deps

import "testing"

func TestSplitImageRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantRepo string
		wantTag  string
		wantOK   bool
	}{
		{"alloy:latest", "alloy", "latest", true},
		{"grafana/alloy:1.2.3", "grafana/alloy", "1.2.3", true},
		// Registry-with-port refs: ":port" must not be mistaken for the tag.
		{"localhost:5000/alloy:dev", "localhost:5000/alloy", "dev", true},
		{"registry.example.com:443/team/app:v1", "registry.example.com:443/team/app", "v1", true},

		{"alloy", "", "", false},
		{"localhost:5000/alloy", "", "", false},
		{"", "", "", false},
		{":latest", "", "", false},
		{"alloy:", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			repo, tag, ok := splitImageRef(tc.ref)
			if ok != tc.wantOK || repo != tc.wantRepo || tag != tc.wantTag {
				t.Fatalf("splitImageRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.ref, repo, tag, ok, tc.wantRepo, tc.wantTag, tc.wantOK)
			}
		})
	}
}
