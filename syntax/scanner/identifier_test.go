package scanner_test

import (
	"testing"

	"github.com/grafana/river/scanner"
	"github.com/stretchr/testify/require"
)

var validTestCases = []struct {
	name       string
	identifier string
	expect     bool
}{
	{"empty", "", false},
	{"start_number", "0identifier_1", false},
	{"start_char", "identifier_1", true},
	{"start_underscore", "_identifier_1", true},
	{"special_chars", "!@#$%^&*()", false},
	{"special_char", "identifier_1!", false},
	{"spaces", "identifier _ 1", false},
}

func TestIsValidIdentifier(t *testing.T) {
	for _, tc := range validTestCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expect, scanner.IsValidIdentifier(tc.identifier))
		})
	}
}

func BenchmarkIsValidIdentifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, tc := range validTestCases {
			_ = scanner.IsValidIdentifier(tc.identifier)
		}
	}
}

var sanitizeTestCases = []struct {
	name             string
	identifier       string
	expectIdentifier string
	expectErr        string
}{
	{"empty", "", "", "cannot generate a new identifier for an empty string"},
	{"start_number", "0identifier_1", "_0identifier_1", ""},
	{"start_char", "identifier_1", "identifier_1", ""},
	{"start_underscore", "_identifier_1", "_identifier_1", ""},
	{"special_chars", "!@#$%^&*()", "__________", ""},
	{"special_char", "identifier_1!", "identifier_1_", ""},
	{"spaces", "identifier _ 1", "identifier___1", ""},
}

func TestSanitizeIdentifier(t *testing.T) {
	for _, tc := range sanitizeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			newIdentifier, err := scanner.SanitizeIdentifier(tc.identifier)
			if tc.expectErr != "" {
				require.EqualError(t, err, tc.expectErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectIdentifier, newIdentifier)
		})
	}
}

func BenchmarkSanitizeIdentifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, tc := range sanitizeTestCases {
			_, _ = scanner.SanitizeIdentifier(tc.identifier)
		}
	}
}

func FuzzSanitizeIdentifier(f *testing.F) {
	for _, tc := range sanitizeTestCases {
		f.Add(tc.identifier)
	}

	f.Fuzz(func(t *testing.T, input string) {
		newIdentifier, err := scanner.SanitizeIdentifier(input)
		if input == "" {
			require.EqualError(t, err, "cannot generate a new identifier for an empty string")
			return
		}
		require.NoError(t, err)
		require.True(t, scanner.IsValidIdentifier(newIdentifier))
	})
}
