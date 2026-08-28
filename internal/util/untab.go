package util

import "strings"

// Untab is a utility function for tests to make it easier
// to write YAML tests, where some editors will insert tabs
// into strings by default.
func Untab(s string) string {
	return strings.ReplaceAll(s, "\t", "  ")
}
