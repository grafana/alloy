package stages

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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

// validateLuhnFilterConfig validates the LuhnFilterConfig.
func validateLuhnFilterConfig(c *LuhnFilterConfig) error {
	if c.Replacement == "" {
		c.Replacement = "**REDACTED**"
	}
	if c.MinLength < 1 {
		c.MinLength = 13
	}
	if c.Source != nil && *c.Source == "" {
		return ErrEmptyRegexStageSource
	}
	return nil
}

// newLuhnFilterStage creates a new LuhnFilterStage.
func newLuhnFilterStage(config LuhnFilterConfig) (Stage, error) {
	if err := validateLuhnFilterConfig(&config); err != nil {
		return nil, err
	}

	var skipRegex *regexp.Regexp
	if config.SkipRegex != "" {
		var err error
		skipRegex, err = regexp.Compile(config.SkipRegex)
		if err != nil {
			return nil, fmt.Errorf("%v: %w", ErrCouldNotCompileRegex, err)
		}
	}

	return toStage(&luhnFilterStage{
		config:    &config,
		skipRegex: skipRegex,
	}), nil
}

// luhnFilterStage applies Luhn algorithm filtering to log entries.
type luhnFilterStage struct {
	config    *LuhnFilterConfig
	skipRegex *regexp.Regexp
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

	// Replace Luhn-valid numbers in the input.
	if r.skipRegex != nil {
		updatedEntry := replaceLuhnValidNumbersSkipRegex(*input, r.config.Replacement, r.config.MinLength, r.config.Delimiters, r.skipRegex)
		*entry = updatedEntry
	} else if r.config.Delimiters != "" {
		updatedEntry := replaceLuhnValidNumbersWithDelimiters(*input, r.config.Replacement, r.config.MinLength, r.config.Delimiters)
		*entry = updatedEntry
	} else {
		updatedEntry := replaceLuhnValidNumbers(*input, r.config.Replacement, r.config.MinLength)
		*entry = updatedEntry
	}
}

// replaceLuhnValidNumbersSkipRegex scans Luhn candidates in order and only evaluates skipRegex
// after finding the first valid candidate. The regex match cursor then only moves forward.
func replaceLuhnValidNumbersSkipRegex(input, replacement string, minLength int, delimiters string, skipRegex *regexp.Regexp) string {
	var output strings.Builder
	output.Grow(len(input))
	var currentNumber strings.Builder
	var trailingDelimiter rune
	numberStart := -1
	numberEnd := -1
	numberTextEnd := -1

	var skipMatches [][]int
	skipMatchIndex := 0
	skipMatchesLoaded := false

	// Checks if the range is skipped by the skipRegex
	isSkipped := func(start, end int) bool {
		if !skipMatchesLoaded { // Lazy load the skipRegex matches
			skipMatches = skipRegex.FindAllStringIndex(input, -1)
			skipMatchesLoaded = true
		}

		for skipMatchIndex < len(skipMatches) && skipMatches[skipMatchIndex][1] <= start {
			skipMatchIndex++
		}
		if skipMatchIndex == len(skipMatches) {
			return false
		}

		match := skipMatches[skipMatchIndex]
		return match[0] <= start && end <= match[1]
	}

	// Flushes the current number to the output, only replacing it if it's a Luhn-valid number and not skipped.
	flushNumber := func() {
		if currentNumber.Len() >= minLength {
			number, err := strconv.Atoi(currentNumber.String())
			if err == nil && isLuhn(number) {
				if isSkipped(numberStart, numberEnd) {
					output.WriteString(input[numberStart:numberTextEnd])
				} else {
					output.WriteString(replacement)
					if trailingDelimiter != 0 {
						output.WriteRune(trailingDelimiter)
					}
				}
			} else {
				output.WriteString(input[numberStart:numberTextEnd])
			}
		} else if currentNumber.Len() > 0 {
			output.WriteString(input[numberStart:numberTextEnd])
		}

		currentNumber.Reset()
		trailingDelimiter = 0
		numberStart = -1
		numberEnd = -1
		numberTextEnd = -1
	}

	// Iterate over the input, replacing Luhn-valid numbers.
	for pos, char := range input {
		switch {
		case unicode.IsDigit(char):
			if numberStart == -1 {
				numberStart = pos
			}
			currentNumber.WriteRune(char)
			numberEnd = pos + utf8.RuneLen(char)
			numberTextEnd = numberEnd
			trailingDelimiter = 0
		case delimiters != "" && strings.ContainsRune(delimiters, char) && currentNumber.Len() > 0:
			numberTextEnd = pos + utf8.RuneLen(char)
			trailingDelimiter = char
		default:
			flushNumber()
			output.WriteRune(char)
		}
	}
	flushNumber()

	return output.String()
}

// replaceLuhnValidNumbers scans the input for Luhn-valid numbers and replaces them.
func replaceLuhnValidNumbers(input, replacement string, minLength int) string {
	var sb strings.Builder
	sb.Grow(len(input))
	var currentNumber strings.Builder

	flushNumber := func() {
		// If the number is at least minLength, check if it's a Luhn-valid number.
		if currentNumber.Len() >= minLength {
			numberStr := currentNumber.String()
			number, err := strconv.Atoi(numberStr)
			if err == nil && isLuhn(number) {
				// If the number is Luhn-valid, replace it.
				sb.WriteString(replacement)
			} else {
				// If the number is not Luhn-valid, write it as is.
				sb.WriteString(numberStr)
			}
		} else if currentNumber.Len() > 0 {
			// If the number is less than minLength but not empty, write it as is.
			sb.WriteString(currentNumber.String())
		}
		// Reset the current number.
		currentNumber.Reset()
	}

	// Iterate over the input, replacing Luhn-valid numbers.
	for _, char := range input {
		// If the character is a digit, add it to the current number.
		if unicode.IsDigit(char) {
			currentNumber.WriteRune(char)
		} else {
			// If the character is not a digit, flush the current number and write the character.
			flushNumber()
			sb.WriteRune(char)
		}
	}
	flushNumber() // Ensure any trailing number is processed

	return sb.String()
}

// replaceLuhnValidNumbersWithDelimiters scans the input for Luhn-valid numbers with delimiter support and replaces them.
// These are separate functions to keep the base case as fast as possible, if no delimiters are needed.
func replaceLuhnValidNumbersWithDelimiters(input, replacement string, minLength int, delimiters string) string {
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
	for _, char := range input {
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
