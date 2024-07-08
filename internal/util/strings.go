package util

import (
	"regexp"
	"strings"
)

// CamelToSnake is a helper function for converting CamelCase to Snake Case
func CamelToSnake(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

// JoinWithTruncation joins a slice of strings with a separator, truncating the middle if the slice is longer
// than maxLength, using abbreviation as a placeholder for the truncated part. The last element of the slice is always
// included in the result. For example: ["1", "2", "3", "4"] with sep=",", maxLength=3 and abbreviation="..." will
// return "1, 2, ..., 4".
func JoinWithTruncation(str []string, sep string, maxLength int, abbreviation string) string {
	if maxLength <= 0 {
		return ""
	}
	if len(str) <= maxLength {
		return strings.Join(str, sep)
	}
	// We know now that len(str) > maxLength >= 1, so we need to truncate something.
	// Handle the special case of maxLength == 1.
	if maxLength == 1 {
		return str[0] + sep + abbreviation
	}
	// We know now that len(str) > maxLength >= 2, can safely truncate the middle.
	result := append(str[:maxLength-1], abbreviation)
	result = append(result, str[len(str)-1])
	return strings.Join(result, sep)
}
