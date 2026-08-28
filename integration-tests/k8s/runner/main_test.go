package main

import "testing"

func TestNormalizeTestTags(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \t  ", ""},
		{"single tag", "gore2regex", "gore2regex"},
		{"space separated", "gore2regex nodocker", "gore2regex,nodocker"},
		{"comma separated", "gore2regex,nodocker", "gore2regex,nodocker"},
		{"mixed separators", "gore2regex, nodocker  extra", "gore2regex,nodocker,extra"},
		{"trailing comma", "gore2regex,", "gore2regex"},
		{"duplicate separators", "gore2regex,,  ,nodocker", "gore2regex,nodocker"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTestTags(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeTestTags(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
