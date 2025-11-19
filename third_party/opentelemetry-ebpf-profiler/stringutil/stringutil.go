// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stringutil // import "go.opentelemetry.io/ebpf-profiler/stringutil"

import (
	"strings"
)

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// FieldsN splits the string s around each instance of one or more consecutive space
// characters, filling f with substrings of s.
// If s contains more fields than n, the last element of f is set to the
// unparsed remainder of s starting with the first non-space character.
// f will stay untouched if s is empty or contains only white space.
// if n is greater than len(f), 0 is returned without doing any parsing.
//
// Apart from the mentioned differences, FieldsN is like an allocation-free strings.Fields.
func FieldsN(s string, f []string) int {
	n := len(f)
	si := 0
	for i := 0; i < n-1; i++ {
		// Find the start of the next field.
		for si < len(s) && asciiSpace[s[si]] != 0 {
			si++
		}
		fieldStart := si

		// Find the end of the field.
		for si < len(s) && asciiSpace[s[si]] == 0 {
			si++
		}
		if fieldStart >= si {
			return i
		}

		f[i] = s[fieldStart:si]
	}

	// Find the start of the next field.
	for si < len(s) && asciiSpace[s[si]] != 0 {
		si++
	}

	// Put the remainder of s as last element of f.
	if si < len(s) {
		f[n-1] = s[si:]
		return n
	}

	return n - 1
}

// SplitN splits the string around each instance of sep, filling f with substrings of s.
// If s contains more fields than n, the last element of f is set to the
// unparsed remainder of s starting with the first non-space character.
// f will stay untouched if s is empty or contains only white space.
// if n is greater than len(f), 0 is returned without doing any parsing.
//
// Apart from the mentioned differences, SplitN is like an allocation-free strings.SplitN.
func SplitN(s, sep string, f []string) int {
	n := len(f)
	i := 0
	for ; i < n-1 && s != ""; i++ {
		fieldEnd := strings.Index(s, sep)
		if fieldEnd < 0 {
			f[i] = s
			return i + 1
		}
		f[i] = s[:fieldEnd]
		s = s[fieldEnd+len(sep):]
	}

	// Put the remainder of s as last element of f.
	f[i] = s
	return i + 1
}
