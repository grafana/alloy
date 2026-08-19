package stages

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
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

func TestLuhnFilterStage(t *testing.T) {
	const (
		uuidRegex     = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`
		nonLuhnUUID   = "a1b2c3d4-e5f6-a1b2-c3d4-e5f6a1b2c3d4"
		luhnUUID      = "a3f1b2e4-c5d6-7e8f-4242-424242424242"
		luhnCard      = "4242424242424242"
		anotherCard   = "6011111111111117"
		delimitedCard = "4242-4242-4242-4242"
		replacement   = "**REDACTED**"
	)

	now := time.Now()

	type testCase struct {
		name     string
		cfg      LuhnFilterConfig
		entries  []Entry
		expected []Entry
	}

	// base is the LuhnFilterConfig shared by every case that doesn't override
	// replacement, min length, delimiters, or skip regex.
	base := LuhnFilterConfig{
		Replacement: replacement,
		MinLength:   12,
		SkipRegex:   uuidRegex,
	}

	tests := []testCase{
		{
			name: "no Luhn number",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "payment accepted", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "payment accepted", now),
			},
		},
		{
			name: "matching UUID without a Luhn number",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+nonLuhnUUID, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+nonLuhnUUID, now),
			},
		},
		{
			name: "standalone Luhn number is redacted",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+replacement, now),
			},
		},
		{
			name: "Luhn number contained by a skip match is preserved",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   `safe-card=[0-9]+`,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "safe-card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "safe-card="+luhnCard, now),
			},
		},
		{
			name: "Luhn number equal to a skip match is preserved",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   luhnCard,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+luhnCard, now),
			},
		},
		{
			name: "Luhn-valid UUID segment is preserved",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+luhnUUID, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+luhnUUID, now),
			},
		},
		{
			name: "card before UUID is redacted",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+luhnCard+" session="+luhnUUID, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+replacement+" session="+luhnUUID, now),
			},
		},
		{
			name: "card after UUID is redacted",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+luhnUUID+" card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+luhnUUID+" card="+replacement, now),
			},
		},
		{
			name: "cursor advances past an earlier non-Luhn UUID",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+nonLuhnUUID+" card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session="+nonLuhnUUID+" card="+replacement, now),
			},
		},
		{
			name: "multiple skip matches and cards are handled in order",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session1="+luhnUUID+" card1="+luhnCard+" session2="+luhnUUID+" card2="+anotherCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "session1="+luhnUUID+" card1="+replacement+" session2="+luhnUUID+" card2="+replacement, now),
			},
		},
		{
			name: "one skip match can contain multiple Luhn numbers",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   `safe=[0-9/]+`,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "safe="+luhnCard+"/"+anotherCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "safe="+luhnCard+"/"+anotherCard, now),
			},
		},
		{
			name: "partial overlap does not suppress redaction",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   `42424242$`,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+replacement, now),
			},
		},
		{
			name: "zero-length skip matches do not suppress redaction",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   `^|$`,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+replacement, now),
			},
		},
		{
			name: "invalid Luhn number is unchanged",
			cfg:  base,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card=4242424242424243", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card=4242424242424243", now),
			},
		},
		{
			name: "Luhn number below minimum length is unchanged",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   13,
				SkipRegex:   uuidRegex,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "number=424242424242", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "number=424242424242", now),
			},
		},
		{
			name: "custom replacement is used",
			cfg: LuhnFilterConfig{
				Replacement: "[SECRET]",
				MinLength:   12,
				SkipRegex:   uuidRegex,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+luhnCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card=[SECRET]", now),
			},
		},
		{
			name: "delimited card is redacted",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				Delimiters:  "-",
				SkipRegex:   uuidRegex,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+delimitedCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "card="+replacement, now),
			},
		},
		{
			name: "delimited card contained by a skip match is preserved",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				Delimiters:  "-",
				SkipRegex:   `safe-card=[0-9-]+`,
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "safe-card="+delimitedCard, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "safe-card="+delimitedCard, now),
			},
		},
		{
			name: "source without a Luhn number replaces the entry",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   uuidRegex,
				Source:      ptr("message"),
			},
			entries: []Entry{
				newEntry(map[string]any{"message": "payment accepted"}, model.LabelSet{}, "original log line", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": "payment accepted"}, model.LabelSet{}, "payment accepted", now),
			},
		},
		{
			name: "source preserves UUID and redacts card",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   uuidRegex,
				Source:      ptr("message"),
			},
			entries: []Entry{
				newEntry(map[string]any{"message": "session=" + luhnUUID + " card=" + luhnCard}, model.LabelSet{}, "original log line", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": "session=" + luhnUUID + " card=" + luhnCard}, model.LabelSet{}, "session="+luhnUUID+" card="+replacement, now),
			},
		},
		{
			name: "skip regex is evaluated against source rather than entry",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   uuidRegex,
				Source:      ptr("message"),
			},
			entries: []Entry{
				newEntry(map[string]any{"message": "card=" + luhnCard}, model.LabelSet{}, "session="+luhnUUID, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": "card=" + luhnCard}, model.LabelSet{}, "card="+replacement, now),
			},
		},
		{
			name: "missing source leaves entry unchanged",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   uuidRegex,
				Source:      ptr("message"),
			},
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "original log line", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "original log line", now),
			},
		},
		{
			name: "non-string source leaves entry unchanged",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				SkipRegex:   uuidRegex,
				Source:      ptr("message"),
			},
			entries: []Entry{
				newEntry(map[string]any{"message": 42}, model.LabelSet{}, "original log line", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": 42}, model.LabelSet{}, "original log line", now),
			},
		},
		{
			name: "source supports delimiters",
			cfg: LuhnFilterConfig{
				Replacement: replacement,
				MinLength:   12,
				Delimiters:  " -",
				SkipRegex:   uuidRegex,
				Source:      ptr("message"),
			},
			entries: []Entry{
				newEntry(map[string]any{"message": "card=" + delimitedCard + " session=" + luhnUUID}, model.LabelSet{}, "original log line", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"message": "card=" + delimitedCard + " session=" + luhnUUID}, model.LabelSet{}, "card="+replacement+" session="+luhnUUID, now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{LuhnFilterConfig: &tt.cfg}}, tt.entries, tt.expected, "")
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
				batch := loki.NewBatch()
				batch.Add(loki.NewStream(model.LabelSet{}, push.Entry{
					Timestamp: time.Now(),
					Line:      fx.entry,
				}))

				cfg := LuhnFilterConfig{
					Replacement: "**REDACTED**",
					MinLength:   12,
					SkipRegex:   sr.skipRegex,
				}
				runPipelineBenchmark(b, []StageConfig{{LuhnFilterConfig: &cfg}}, batch)
			})
		}
	}
}

func TestValidateLuhnFilterConfig(t *testing.T) {
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

func TestLuhnFilterStageRejectsInvalidSkipRegex(t *testing.T) {
	_, err := newLuhnFilterStage(LuhnFilterConfig{SkipRegex: "("}, nil)
	require.ErrorContains(t, err, errCouldNotCompileRegex.Error())
}
