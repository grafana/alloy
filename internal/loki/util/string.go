package util

import (
	"bytes"
	"fmt"
	"unicode"
)

// SnakeCase converts given string `s` into `snake_case`.
func SnakeCase(s string) string {
	var buf bytes.Buffer
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 && s[i-1] != '_' {
			fmt.Fprintf(&buf, "_")
		}
		r = unicode.ToLower(r)
		fmt.Fprintf(&buf, "%c", r)
	}
	return buf.String()
}
