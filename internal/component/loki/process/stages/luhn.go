package stages

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/prometheus/common/model"
)

// LuhnFilterConfig configures a processing stage that filters out Luhn-valid numbers.
type LuhnFilterConfig struct {
	Replacement string  `alloy:"replacement,attr,optional"`
	Source      *string `alloy:"source,attr,optional"`
	MinLength   int     `alloy:"min_length,attr,optional"`
	Delimiters  string  `alloy:"delimiters,attr,optional"`
	SkipRegex   string  `alloy:"skip_regex,attr,optional"`
}

// validateLuhnFilterConfig validates the LuhnFilterConfig and returns the compiled
// skip_regex expression, if one was configured.
func validateLuhnFilterConfig(c *LuhnFilterConfig) (*regexp.Regexp, error) {
	if c.Replacement == "" {
		c.Replacement = "**REDACTED**"
	}
	if c.MinLength < 1 {
		c.MinLength = 13
	}
	if c.Source != nil && *c.Source == "" {
		return nil, ErrEmptyRegexStageSource
	}
	if c.SkipRegex == "" {
		return nil, nil
	}
	skipRegex, err := regexp.Compile(c.SkipRegex)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", ErrCouldNotCompileRegex, err)
	}
	return skipRegex, nil
}

// newLuhnFilterStage creates a new LuhnFilterStage.
func newLuhnFilterStage(config LuhnFilterConfig) (Stage, error) {
	skipRegex, err := validateLuhnFilterConfig(&config)
	if err != nil {
		return nil, err
	}
	stage := &luhnFilterStage{config: &config}
	if skipRegex != nil {
		stage.skipRegex = skipRegex
	}
	return toStage(stage), nil
}

// skipRegexMatcher is the subset of *regexp.Regexp that findSkipRanges needs. It exists so
// tests can substitute a call-counting stub to verify the regex is only run when needed.
type skipRegexMatcher interface {
	FindAllStringIndex(s string, n int) [][]int
}

// luhnFilterStage applies Luhn algorithm filtering to log entries.
type luhnFilterStage struct {
	config    *LuhnFilterConfig
	skipRegex skipRegexMatcher
}

// Process implements Stage.
func (r *luhnFilterStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	input := entry
	if r.config.Source != nil {
		value, ok := extracted[*r.config.Source]
		if !ok {
			return
		}
		strVal, ok := value.(string)
		if !ok {
			return
		}
		input = &strVal
	}

	if input == nil {
		return
	}

	var skipRanges [][]int
	if r.skipRegex != nil {
		// Running skip_regex is only worthwhile if there's a Luhn-valid number to potentially
		// exempt from redaction. Skip it entirely for the common case of no match at all.
		var hasMatch bool
		if r.config.Delimiters != "" {
			hasMatch = hasLuhnValidNumberWithDelimiters(*input, r.config.MinLength, r.config.Delimiters)
		} else {
			hasMatch = hasLuhnValidNumber(*input, r.config.MinLength)
		}
		if !hasMatch {
			return
		}
		skipRanges = findSkipRanges(*input, r.skipRegex)
	}

	if r.config.Delimiters != "" {
		*entry = replaceLuhnValidNumbersWithDelimiters(*input, r.config.Replacement, r.config.MinLength, r.config.Delimiters, skipRanges)
	} else {
		*entry = replaceLuhnValidNumbers(*input, r.config.Replacement, r.config.MinLength, skipRanges)
	}
}

// hasLuhnValidNumber reports whether input contains a run of at least minLength digits that is
// Luhn-valid. It mirrors the digit-scanning logic in replaceLuhnValidNumbers without building any
// output, so it can cheaply decide whether skip_regex is worth evaluating.
func hasLuhnValidNumber(input string, minLength int) bool {
	digitStart := -1

	checkRun := func(end int) bool {
		if digitStart == -1 {
			return false
		}
		start := digitStart
		digitStart = -1
		if end-start < minLength {
			return false
		}
		number, err := strconv.Atoi(input[start:end])
		return err == nil && isLuhn(number)
	}

	for i, char := range input {
		if unicode.IsDigit(char) {
			if digitStart == -1 {
				digitStart = i
			}
		} else if checkRun(i) {
			return true
		}
	}
	return checkRun(len(input))
}

// hasLuhnValidNumberWithDelimiters is the delimiter-aware counterpart to hasLuhnValidNumber,
// mirroring replaceLuhnValidNumbersWithDelimiters's digit-scanning logic.
func hasLuhnValidNumberWithDelimiters(input string, minLength int, delimiters string) bool {
	var currentNumber strings.Builder

	checkRun := func() bool {
		valid := false
		if currentNumber.Len() >= minLength {
			number, err := strconv.Atoi(currentNumber.String())
			valid = err == nil && isLuhn(number)
		}
		currentNumber.Reset()
		return valid
	}

	for _, char := range input {
		switch {
		case unicode.IsDigit(char):
			currentNumber.WriteRune(char)
		case delimiters != "" && strings.ContainsRune(delimiters, char) && currentNumber.Len() > 0:
			continue
		default:
			if checkRun() {
				return true
			}
		}
	}
	return checkRun()
}

// findSkipRanges returns the byte ranges of all matches of skipRegex found in input, in the same
// [start, end] pair-per-match form FindAllStringIndex already produces.
func findSkipRanges(input string, skipRegex skipRegexMatcher) [][]int {
	return skipRegex.FindAllStringIndex(input, -1)
}

// newSkipRangeCursor returns a function that reports whether a byte position falls within any of
// ranges. ranges must be sorted and non-overlapping (as returned by FindAllStringIndex), and
// successive calls must pass strictly increasing positions (as in a range-over-string loop). Under
// those conditions the cursor advances forward only, giving O(n+m) total cost across a scan instead
// of the O(n*m) a fresh per-character linear scan over ranges would cost.
func newSkipRangeCursor(ranges [][]int) func(pos int) bool {
	idx := 0
	return func(pos int) bool {
		for idx < len(ranges) && pos >= ranges[idx][1] {
			idx++
		}
		return idx < len(ranges) && pos >= ranges[idx][0]
	}
}

// replaceLuhnValidNumbers scans the input for Luhn-valid numbers and replaces them.
func replaceLuhnValidNumbers(input, replacement string, minLength int, skipRanges [][]int) string {
	var sb strings.Builder
	sb.Grow(len(input))
	// Track the current digit run by its start offset into input rather than copying digits into
	// a separate buffer. This avoids an allocation per run, which matters when digit runs are short
	// and frequent (e.g. lone digits interspersed with hex letters in a UUID).
	digitStart := -1

	flushNumber := func(end int) {
		if digitStart == -1 {
			return
		}
		start := digitStart
		digitStart = -1
		if end-start < minLength {
			// If the number is less than minLength but not empty, write it as is.
			sb.WriteString(input[start:end])
			return
		}
		// If the number is at least minLength, check if it's a Luhn-valid number.
		number, err := strconv.Atoi(input[start:end])
		if err == nil && isLuhn(number) {
			// If the number is Luhn-valid, replace it.
			sb.WriteString(replacement)
		} else {
			// If the number is not Luhn-valid, write it as is.
			sb.WriteString(input[start:end])
		}
	}

	// Iterate over the input, replacing Luhn-valid numbers.
	inSkipRange := newSkipRangeCursor(skipRanges)
	for pos, char := range input {
		// Characters that fall inside a skip_regex match are passed through unchanged.
		if inSkipRange(pos) {
			flushNumber(pos)
			sb.WriteRune(char)
			continue
		}
		// If the character is a digit, extend the current run.
		if unicode.IsDigit(char) {
			if digitStart == -1 {
				digitStart = pos
			}
		} else {
			// If the character is not a digit, flush the current run and write the character.
			flushNumber(pos)
			sb.WriteRune(char)
		}
	}
	flushNumber(len(input)) // Ensure any trailing number is processed

	return sb.String()
}

// replaceLuhnValidNumbersWithDelimiters scans the input for Luhn-valid numbers with delimiter support and replaces them.
// These are separate functions to keep the base case as fast as possible, if no delimiters are needed.
func replaceLuhnValidNumbersWithDelimiters(input, replacement string, minLength int, delimiters string, skipRanges [][]int) string {
	var sb strings.Builder
	sb.Grow(len(input))
	var currentNumber strings.Builder
	var currentString strings.Builder
	var trailingDelimiter rune

	flushNumber := func() {
		// If the number is at least minLength, check if it's a Luhn-valid number.
		if currentNumber.Len() >= minLength {
			numberStr := currentNumber.String()
			number, err := strconv.Atoi(numberStr)
			if err == nil && isLuhn(number) {
				// If the number is Luhn-valid, replace it.
				sb.WriteString(replacement)
				if trailingDelimiter != 0 {
					sb.WriteRune(trailingDelimiter)
				}
			} else {
				// If the number is not Luhn-valid, write it as is.
				sb.WriteString(currentString.String())
			}
		} else if currentNumber.Len() > 0 {
			// If the number is less than minLength but not empty, write it as is.
			sb.WriteString(currentString.String())
		}
		// Reset the current tracking.
		currentNumber.Reset()
		currentString.Reset()
		trailingDelimiter = 0
	}

	// Iterate over the input, replacing Luhn-valid numbers.
	inSkipRange := newSkipRangeCursor(skipRanges)
	for pos, char := range input {
		// Characters that fall inside a skip_regex match are passed through unchanged.
		if inSkipRange(pos) {
			flushNumber()
			sb.WriteRune(char)
			continue
		}
		// If the character is a digit, add it to the current number.
		if unicode.IsDigit(char) {
			currentNumber.WriteRune(char)
			currentString.WriteRune(char)
			trailingDelimiter = 0
		} else if delimiters != "" && strings.ContainsRune(delimiters, char) && currentNumber.Len() > 0 {
			currentString.WriteRune(char)
			trailingDelimiter = char
			// If the character is a delimiter and we have a current number, skip the delimiter.
			// This way we can capture credit card numbers for example with spaces or dashes in between.
			continue
		} else {
			// If the character is not a digit, flush the current number and write the character.
			flushNumber()
			sb.WriteRune(char)
		}
	}
	flushNumber() // Ensure any trailing number is processed

	return sb.String()
}

// isLuhn check number is valid or not based on Luhn algorithm
func isLuhn(number int) bool {
	// Luhn algorithm is a simple checksum formula used to validate a
	// variety of identification numbers, such as credit card numbers, IMEI
	// numbers, National Provider Identifier numbers in the US, and
	// Canadian Social Insurance Numbers. This is a simple implementation
	// of the Luhn algorithm.
	// https://en.wikipedia.org/wiki/Luhn_algorithm
	return (number%10+checksum(number/10))%10 == 0
}

func checksum(number int) int {
	var luhn int

	for i := 0; number > 0; i++ {
		cur := number % 10

		if i%2 == 0 { // even
			cur *= 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}

		luhn += cur
		number /= 10
	}
	return luhn % 10
}
