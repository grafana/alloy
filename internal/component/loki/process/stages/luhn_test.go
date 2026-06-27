package stages

import (
	"regexp"
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
			got = replaceLuhnValidNumbers(c.input, c.replacement, 13, nil)
		} else {
			got = replaceLuhnValidNumbersWithDelimiters(c.input, c.replacement, 13, c.delimiters, nil)
		}
		if got != c.want {
			t.Errorf("replaceLuhnValidNumbers(%q, %q) == %q, want %q", c.input, c.replacement, got, c.want)
		}
	}
}

func TestSkipRegex(t *testing.T) {
	const (
		uuidRegexStr = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`
		luhnUUID     = "a3f1b2e4-c5d6-7e8f-4242-424242424242"
		luhnCC       = "4242424242424242" // 16-digit Luhn-valid credit card number (Stripe test card)
		replacement  = "**REDACTED**"
	)
	uuidRegex := regexp.MustCompile(uuidRegexStr)

	t.Run("findSkipRanges finds a single match", func(t *testing.T) {
		input := "session=" + luhnUUID + " end"
		ranges := findSkipRanges(input, uuidRegex)
		require.Len(t, ranges, 1)
		require.Equal(t, [2]int{8, 8 + len(luhnUUID)}, ranges[0])
	})

	t.Run("findSkipRanges returns nil when no matches present", func(t *testing.T) {
		require.Nil(t, findSkipRanges("no uuids here "+luhnCC, uuidRegex))
	})

	t.Run("findSkipRanges finds multiple matches", func(t *testing.T) {
		input := luhnUUID + " " + luhnUUID
		require.Len(t, findSkipRanges(input, uuidRegex), 2)
	})

	t.Run("UUID Luhn-valid segment preserved when skip_regex matches it", func(t *testing.T) {
		// With minLength=12, the last UUID segment (424242424242) would be redacted normally.
		input := "session=" + luhnUUID
		got := replaceLuhnValidNumbers(input, replacement, 12, findSkipRanges(input, uuidRegex))
		require.Equal(t, input, got)
	})

	t.Run("UUID Luhn-valid segment redacted when skip_regex not configured", func(t *testing.T) {
		// Baseline: without skip ranges the segment gets replaced.
		input := "session=" + luhnUUID
		got := replaceLuhnValidNumbers(input, replacement, 12, nil)
		require.Contains(t, got, replacement)
		require.NotContains(t, got, "424242424242")
	})

	t.Run("credit card redacted but UUID preserved", func(t *testing.T) {
		input := "card=" + luhnCC + " session=" + luhnUUID
		got := replaceLuhnValidNumbers(input, replacement, 12, findSkipRanges(input, uuidRegex))
		require.Contains(t, got, replacement)
		require.Contains(t, got, luhnUUID)
		require.NotContains(t, got, luhnCC)
	})

	t.Run("Luhn detection still works when skip_regex configured but no matches present", func(t *testing.T) {
		input := "card=" + luhnCC
		got := replaceLuhnValidNumbers(input, replacement, 16, findSkipRanges(input, uuidRegex))
		require.Contains(t, got, replacement)
		require.NotContains(t, got, luhnCC)
	})

	t.Run("delimiter support preserves UUID", func(t *testing.T) {
		input := "card=4242-4242-4242-4242 session=" + luhnUUID
		got := replaceLuhnValidNumbersWithDelimiters(input, replacement, 16, " -", findSkipRanges(input, uuidRegex))
		require.Contains(t, got, replacement)
		require.Contains(t, got, luhnUUID)
		require.NotContains(t, got, "4242-4242-4242-4242")
	})

	t.Run("end-to-end Process with skip_regex set to the uuid pattern", func(t *testing.T) {
		input := "card=" + luhnCC + " session=" + luhnUUID
		entry := input
		stage := &luhnFilterStage{
			config: &LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   uuidRegexStr,
			},
			skipRegex: uuidRegex,
		}
		stage.Process(nil, nil, nil, &entry)
		require.Contains(t, entry, replacement)
		require.Contains(t, entry, luhnUUID)
		require.NotContains(t, entry, luhnCC)
	})

	t.Run("end-to-end Process with skip_regex unset leaves behavior unchanged", func(t *testing.T) {
		input := "card=" + luhnCC + " session=" + luhnUUID
		entry := input
		stage := &luhnFilterStage{
			config: &LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
			},
		}
		stage.Process(nil, nil, nil, &entry)
		require.Contains(t, entry, replacement)
		require.NotContains(t, entry, luhnCC)
		require.NotContains(t, entry, luhnUUID)
	})

	t.Run("newLuhnFilterStage compiles skip_regex from config", func(t *testing.T) {
		stage, err := newLuhnFilterStage(LuhnFilterConfig{
			Replacement: replacement,
			MinLength:   12,
			SkipRegex:   uuidRegexStr,
		})
		require.NoError(t, err)

		entry := "card=" + luhnCC + " session=" + luhnUUID
		stage.(Processor).Process(nil, nil, nil, &entry)
		require.Contains(t, entry, replacement)
		require.Contains(t, entry, luhnUUID)
		require.NotContains(t, entry, luhnCC)
	})

	t.Run("newLuhnFilterStage rejects invalid skip_regex", func(t *testing.T) {
		_, err := newLuhnFilterStage(LuhnFilterConfig{
			Replacement: replacement,
			MinLength:   12,
			SkipRegex:   "(",
		})
		require.Error(t, err)
	})
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
		{
			name: "invalid skip_regex error",
			input: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &source,
				MinLength:   10,
				SkipRegex:   "(",
			},
			expected: LuhnFilterConfig{
				Replacement: "ABC",
				Source:      &source,
				MinLength:   10,
				SkipRegex:   "(",
			},
			errorContainsStr: "could not compile",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := validateLuhnFilterConfig(&c.input)
			if c.errorContainsStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, c.errorContainsStr)
			}
			require.Equal(t, c.expected, c.input)
		})
	}
}
