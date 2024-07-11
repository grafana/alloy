package util

import (
	"regexp"
	"strings"

	"k8s.io/utils/strings/slices"
)

// CamelToSnake is a helper function for converting CamelCase to Snake Case
func CamelToSnake(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

// JoinWithTruncation joins a slice of string elements with a separator sep, truncating the middle if the slice is longer
// than maxElements, using abbreviation as a placeholder for the truncated part. The last element of the slice is always
// included in the result. For example: ["1", "2", "3", "4"] with sep=",", maxLength=3 and abbreviation="..." will
// return "1, 2, ..., 4".
func JoinWithTruncation(elements []string, sep string, maxElements int, abbreviation string) string {
	if maxElements <= 0 {
		return ""
	}
	if len(elements) <= maxElements {
		return strings.Join(elements, sep)
	}
	// We know now that len(elements) > maxElements >= 1, so we need to truncate something.
	// Handle the special case of maxElements == 1.
	if maxElements == 1 {
		return elements[0] + sep + abbreviation
	}
	// We know now that len(elements) > maxElements >= 2, can safely truncate the middle.
	result := slices.Clone(elements[:maxElements-1])
	result = append(result, abbreviation, elements[len(elements)-1])
	return strings.Join(result, sep)
}
