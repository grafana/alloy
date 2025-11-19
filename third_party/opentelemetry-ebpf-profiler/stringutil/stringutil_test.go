// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stringutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFieldsN(t *testing.T) {
	tests := map[string]struct {
		input     string
		expected  []string
		maxFields int
	}{
		"empty":          {"", []string{}, 2},
		"only spaces":    {"  ", []string{}, 2},
		"1 field":        {"111", []string{"111"}, 2},
		"1 field B":      {" 111", []string{"111"}, 2},
		"1 field C":      {"111 ", []string{"111"}, 2},
		"1 field D":      {" 111 ", []string{"111"}, 2},
		"2 fields":       {"111 222", []string{"111", "222"}, 2},
		"3 fields cap 2": {"111 222  333", []string{"111", "222  333"}, 2},
		"3 fields cap 3": {"111 222  333", []string{"111", "222", "333"}, 3},
		"4 fields cap 2": {"111 222  333 444", []string{"111", "222  333 444"}, 2},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			var fields [4]string
			n := FieldsN(testcase.input, fields[:testcase.maxFields])
			require.Equal(t, testcase.expected, fields[:n])
		})
	}
}

func TestSplitN(t *testing.T) {
	tests := map[string]struct {
		input     string
		expected  []string
		maxFields int
	}{
		"empty":          {"", []string{""}, 2},
		"only sep":       {"-", []string{"", ""}, 2},
		"1 field":        {"111", []string{"111"}, 2},
		"2 fields B":     {"-111", []string{"", "111"}, 2},
		"2 fields C":     {"111-", []string{"111", ""}, 2},
		"3 fields A":     {"-111-", []string{"", "111", ""}, 3},
		"3 fields B":     {"111-222", []string{"111", "222"}, 3},
		"4 fields cap 3": {"111-222--333", []string{"111", "222", "-333"}, 3},
		"4 fields cap 4": {"111-222--333", []string{"111", "222", "", "333"}, 4},
		"5 fields cap 3": {"111-222--333-444", []string{"111", "222", "-333-444"}, 3},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			var fields [4]string
			n := SplitN(testcase.input, "-", fields[:testcase.maxFields])
			require.Equal(t, testcase.expected, fields[:n])
		})
	}
}
