package scanner

import (
	"fmt"

	"github.com/grafana/river/token"
)

// IsValidIdentifier returns true if the given string is a valid river
// identifier.
func IsValidIdentifier(in string) bool {
	s := New(token.NewFile(""), []byte(in), nil, 0)
	_, tok, lit := s.Scan()
	return tok == token.IDENT && lit == in
}

// SanitizeIdentifier will return the given string mutated into a valid river
// identifier. If the given string is already a valid identifier, it will be
// returned unchanged.
//
// This should be used with caution since the different inputs can result in
// identical outputs.
func SanitizeIdentifier(in string) (string, error) {
	if in == "" {
		return "", fmt.Errorf("cannot generate a new identifier for an empty string")
	}

	if IsValidIdentifier(in) {
		return in, nil
	}

	newValue := generateNewIdentifier(in)
	if !IsValidIdentifier(newValue) {
		panic(fmt.Errorf("invalid identifier %q generated for `%q`", newValue, in))
	}

	return newValue, nil
}

// generateNewIdentifier expects a valid river prefix and replacement
// string and returns a new identifier based on the given input.
func generateNewIdentifier(in string) string {
	newValue := ""
	for i, c := range in {
		if i == 0 {
			if isDigit(c) {
				newValue = "_"
			}
		}

		if !(isLetter(c) || isDigit(c)) {
			newValue += "_"
			continue
		}

		newValue += string(c)
	}

	return newValue
}
