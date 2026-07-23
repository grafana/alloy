package stages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test cases for the Luhn algorithm validation
func TestIsLuhnValid(t *testing.T) {
	cases := []struct {
		input int
		want  bool
	}{
		{4242_4242_4242_4242, true}, // Valid Luhn number
		{1234_5678_1234_5670, true}, // Another valid Luhn number
		{499_2739_8112_1717, false}, // Invalid Luhn number
		{1234567812345678, false},   // Another invalid Luhn number
		{3782_822463_10005, true},   // Short, valid Luhn number
		{123, false},                // Short, invalid Luhn number
	}

	for _, c := range cases {
		got := isLuhn(c.input)
		if got != c.want {
			t.Errorf("isLuhnValid(%d) == %t, want %t", c.input, got, c.want)
		}
	}
}

// TestReplaceLuhnValidNumbers tests the replaceLuhnValidNumbers function.
func TestReplaceLuhnValidNumbers(t *testing.T) {
	cases := []struct {
		input       string
		replacement string
		want        string
		delimiters  string
	}{
		// Test case with a single Luhn-valid number
		{"My credit card number is 3530111333300000.", "**REDACTED**", "My credit card number is **REDACTED**.", ""},
		// Test case with multiple Luhn-valid numbers
		{"Cards 4532015112830366 and 6011111111111117 are valid.", "**REDACTED**", "Cards **REDACTED** and **REDACTED** are valid.", ""},
		// Test case with no Luhn-valid numbers
		{"No valid numbers here.", "**REDACTED**", "No valid numbers here.", ""},
		// Test case with mixed content
		{"Valid: 4556737586899855, invalid: 1234.", "**REDACTED**", "Valid: **REDACTED**, invalid: 1234.", ""},
		// Test case with edge cases
		{"Edge cases: 0, 00, 000, 1.", "**REDACTED**", "Edge cases: 0, 00, 000, 1.", ""},
		// multiple luhns with different delimiters and trailing delimiter
		{"Cards 4532-0151-1283-0366 and 6011 1111 1111 1117 are valid and 3530:1113:3330:0000 has unexpected delimiters.", "**REDACTED**", "Cards **REDACTED** and **REDACTED** are valid and 3530:1113:3330:0000 has unexpected delimiters.", " -"},
		// luhn with delimiters but not valid
		{"Card 4532-0151-1283-0367 is not valid.", "**REDACTED**", "Card 4532-0151-1283-0367 is not valid.", " -"},
		// luhn with delimiters but below min length
		{"Card 4532-0151-128 is too short.", "**REDACTED**", "Card 4532-0151-128 is too short.", "-"},
		// luhn with delimiters but below min length with trailing delimiter
		{"Card 4532-0151-128 is too short.", "**REDACTED**", "Card 4532-0151-128 is too short.", " -"},
	}

	for _, c := range cases {
		var got string
		if c.delimiters == "" {
			got = replaceLuhnValidNumbers(c.input, c.replacement, 13)
		} else {
			got = replaceLuhnValidNumbersWithDelimiters(c.input, c.replacement, 13, c.delimiters)
		}
		if got != c.want {
			t.Errorf("replaceLuhnValidNumbers(%q, %q) == %q, want %q", c.input, c.replacement, got, c.want)
		}
	}
}

func TestLuhnFilterStageRejectsInvalidSkipRegex(t *testing.T) {
	_, err := newLuhnFilterStage(LuhnFilterConfig{SkipRegex: "("})
	require.ErrorContains(t, err, ErrCouldNotCompileRegex.Error())
}

func TestLuhnFilterStageSkipRegexEndToEnd(t *testing.T) {
	const (
		uuidRegex     = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`
		nonLuhnUUID   = "a1b2c3d4-e5f6-a1b2-c3d4-e5f6a1b2c3d4"
		luhnUUID      = "a3f1b2e4-c5d6-7e8f-4242-424242424242"
		luhnCard      = "4242424242424242"
		anotherCard   = "6011111111111117"
		delimitedCard = "4242-4242-4242-4242"
		replacement   = "**REDACTED**"
	)

	tests := []struct {
		name        string
		entry       string
		extracted   map[string]any
		source      string
		replacement string
		minLength   int
		delimiters  string
		skipRegex   string
		want        string
	}{
		{
			name:  "no Luhn number",
			entry: "payment accepted",
			want:  "payment accepted",
		},
		{
			name:  "matching UUID without a Luhn number",
			entry: "session=" + nonLuhnUUID,
			want:  "session=" + nonLuhnUUID,
		},
		{
			name:  "standalone Luhn number is redacted",
			entry: "card=" + luhnCard,
			want:  "card=" + replacement,
		},
		{
			name:      "Luhn number contained by a skip match is preserved",
			entry:     "safe-card=" + luhnCard,
			skipRegex: `safe-card=[0-9]+`,
			want:      "safe-card=" + luhnCard,
		},
		{
			name:      "Luhn number equal to a skip match is preserved",
			entry:     "card=" + luhnCard,
			skipRegex: luhnCard,
			want:      "card=" + luhnCard,
		},
		{
			name:  "Luhn-valid UUID segment is preserved",
			entry: "session=" + luhnUUID,
			want:  "session=" + luhnUUID,
		},
		{
			name:  "card before UUID is redacted",
			entry: "card=" + luhnCard + " session=" + luhnUUID,
			want:  "card=" + replacement + " session=" + luhnUUID,
		},
		{
			name:  "card after UUID is redacted",
			entry: "session=" + luhnUUID + " card=" + luhnCard,
			want:  "session=" + luhnUUID + " card=" + replacement,
		},
		{
			name:  "cursor advances past an earlier non-Luhn UUID",
			entry: "session=" + nonLuhnUUID + " card=" + luhnCard,
			want:  "session=" + nonLuhnUUID + " card=" + replacement,
		},
		{
			name:  "multiple skip matches and cards are handled in order",
			entry: "session1=" + luhnUUID + " card1=" + luhnCard + " session2=" + luhnUUID + " card2=" + anotherCard,
			want:  "session1=" + luhnUUID + " card1=" + replacement + " session2=" + luhnUUID + " card2=" + replacement,
		},
		{
			name:      "one skip match can contain multiple Luhn numbers",
			entry:     "safe=" + luhnCard + "/" + anotherCard,
			skipRegex: `safe=[0-9/]+`,
			want:      "safe=" + luhnCard + "/" + anotherCard,
		},
		{
			name:      "partial overlap does not suppress redaction",
			entry:     "card=" + luhnCard,
			skipRegex: `42424242$`,
			want:      "card=" + replacement,
		},
		{
			name:      "zero-length skip matches do not suppress redaction",
			entry:     "card=" + luhnCard,
			skipRegex: `^|$`,
			want:      "card=" + replacement,
		},
		{
			name:  "invalid Luhn number is unchanged",
			entry: "card=4242424242424243",
			want:  "card=4242424242424243",
		},
		{
			name:      "Luhn number below minimum length is unchanged",
			entry:     "number=424242424242",
			minLength: 13,
			want:      "number=424242424242",
		},
		{
			name:        "custom replacement is used",
			entry:       "card=" + luhnCard,
			replacement: "[SECRET]",
			want:        "card=[SECRET]",
		},
		{
			name:       "delimited card is redacted",
			entry:      "card=" + delimitedCard,
			delimiters: "-",
			want:       "card=" + replacement,
		},
		{
			name:       "delimited card contained by a skip match is preserved",
			entry:      "safe-card=" + delimitedCard,
			delimiters: "-",
			skipRegex:  `safe-card=[0-9-]+`,
			want:       "safe-card=" + delimitedCard,
		},
		{
			name:      "source without a Luhn number replaces the entry",
			entry:     "original log line",
			extracted: map[string]any{"message": "payment accepted"},
			source:    "message",
			want:      "payment accepted",
		},
		{
			name:      "source preserves UUID and redacts card",
			entry:     "original log line",
			extracted: map[string]any{"message": "session=" + luhnUUID + " card=" + luhnCard},
			source:    "message",
			want:      "session=" + luhnUUID + " card=" + replacement,
		},
		{
			name:      "skip regex is evaluated against source rather than entry",
			entry:     "session=" + luhnUUID,
			extracted: map[string]any{"message": "card=" + luhnCard},
			source:    "message",
			want:      "card=" + replacement,
		},
		{
			name:      "missing source leaves entry unchanged",
			entry:     "original log line",
			extracted: map[string]any{},
			source:    "message",
			want:      "original log line",
		},
		{
			name:      "non-string source leaves entry unchanged",
			entry:     "original log line",
			extracted: map[string]any{"message": 42},
			source:    "message",
			want:      "original log line",
		},
		{
			name:       "source supports delimiters",
			entry:      "original log line",
			extracted:  map[string]any{"message": "card=" + delimitedCard + " session=" + luhnUUID},
			source:     "message",
			delimiters: " -",
			want:       "card=" + replacement + " session=" + luhnUUID,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				Delimiters:  tc.delimiters,
				SkipRegex:   uuidRegex,
			}
			if tc.replacement != "" {
				config.Replacement = tc.replacement
			}
			if tc.minLength != 0 {
				config.MinLength = tc.minLength
			}
			if tc.skipRegex != "" {
				config.SkipRegex = tc.skipRegex
			}
			if tc.source != "" {
				config.Source = &tc.source
			}

			stage, err := newLuhnFilterStage(config)
			require.NoError(t, err)

			entry := tc.entry
			stage.(Processor).Process(nil, tc.extracted, nil, &entry)
			require.Equal(t, tc.want, entry)
		})
	}
}

// BenchmarkLuhnFilterStage compares Process performance with skip_regex enabled
// vs disabled, across inputs that do and don't contain UUIDs and Luhn-valid numbers.
func BenchmarkLuhnFilterStage(b *testing.B) {
	const (
		uuidRegexStr = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`
		nonLuhnUUID  = "a1b2c3d4-e5f6-a1b2-c3d4-e5f6a1b2c3d4"
		luhnUUID     = "a3f1b2e4-c5d6-7e8f-4242-424242424242" // last group is a 12-digit Luhn-valid run
		luhnCC       = "4242424242424242"                     // 16-digit Luhn-valid credit card number
	)

	fixtures := []struct {
		name  string
		entry string
	}{
		{
			name:  "no_uuid_no_luhn",
			entry: `level=info ts=2024-01-15T10:23:45Z msg="processing request" request_id=req-8f14e45fceaa user_id=usr-2ab3c9d1e502 note="ref 12345 67890" status=success duration_ms=42`,
		},
		{
			name:  "no_uuid_with_luhn",
			entry: `level=info ts=2024-01-15T10:23:45Z msg="processing payment" request_id=req-8f14e45fceaa user_id=usr-2ab3c9d1e502 card=` + luhnCC + ` status=success duration_ms=42`,
		},
		{
			name:  "with_uuid_no_luhn",
			entry: `level=info ts=2024-01-15T10:23:45Z msg="processing request" request_id=` + nonLuhnUUID + ` user_id=` + nonLuhnUUID + ` note="ref 12345 67890" status=success duration_ms=42`,
		},
		{
			name:  "with_uuid_with_luhn",
			entry: `level=info ts=2024-01-15T10:23:45Z msg="processing payment" request_id=` + luhnUUID + ` user_id=` + luhnUUID + ` card=` + luhnCC + ` status=success duration_ms=42`,
		},
	}

	skipRegexStates := []struct {
		name      string
		skipRegex string
	}{
		{"skip_regex=off", ""},
		{"skip_regex=on", uuidRegexStr},
	}

	for _, fx := range fixtures {
		for _, sr := range skipRegexStates {
			b.Run(fx.name+"/"+sr.name, func(b *testing.B) {
				stage, err := newLuhnFilterStage(LuhnFilterConfig{
					Replacement: "**REDACTED**",
					MinLength:   12,
					SkipRegex:   sr.skipRegex,
				})
				require.NoError(b, err)
				processor := stage.(Processor)

				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					entry := fx.entry
					processor.Process(nil, nil, nil, &entry)
				}
			})
		}
	}
}

func TestValidateConfig(t *testing.T) {
	source := ".*"
	emptySource := ""
	cases := []struct {
		name             string
		input            LuhnFilterConfig
		expected         LuhnFilterConfig
		errorContainsStr string
	}{
		{
			name: "successful validation",
			input: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &source,
				MinLength:   10,
			},
			expected: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &source,
				MinLength:   10,
			},
		},
		{
			name: "nil source",
			input: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      nil,
				MinLength:   10,
			},
			expected: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      nil,
				MinLength:   10,
			},
		},
		{
			name: "empty source error",
			input: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &emptySource,
				MinLength:   11,
			},
			expected: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &emptySource,
				MinLength:   11,
			},
			errorContainsStr: "empty source",
		},
		{
			name: "defaults update",
			input: LuhnFilterConfig{
				Replacement: "",
				Source:      &source,
				MinLength:   -10,
			},
			expected: LuhnFilterConfig{
				Replacement: "**REDACTED**",
				Source:      &source,
				MinLength:   13,
			},
		},
		{
			name: "valid skip_regex",
			input: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &source,
				MinLength:   10,
				SkipRegex:   `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
			},
			expected: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &source,
				MinLength:   10,
				SkipRegex:   `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateLuhnFilterConfig(&c.input)
			if c.errorContainsStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, c.errorContainsStr)
			}
			require.Equal(t, c.expected, c.input)
		})
	}
}
