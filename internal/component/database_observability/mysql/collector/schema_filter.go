package collector

import "strings"

// SchemaFilter selects schemas by name. A schema is selected when it matches
// one of the Include patterns (or Include is empty) and none of the Exclude
// patterns.
//
// A pattern is matched against the whole schema name. '%' matches any run of
// characters, including none; every other character, '_' included, matches
// itself, so "hg_%" does not match "hgwarm_1".
type SchemaFilter struct {
	Include []string
	Exclude []string
}

// Restricted reports whether the filter can leave out any schema.
func (f SchemaFilter) Restricted() bool {
	return len(f.Include) > 0 || len(f.Exclude) > 0
}

// Match reports whether schema is selected.
func (f SchemaFilter) Match(schema string) bool {
	if len(f.Include) > 0 && !matchesAny(f.Include, schema) {
		return false
	}
	return !matchesAny(f.Exclude, schema)
}

func matchesAny(patterns []string, s string) bool {
	for _, p := range patterns {
		if matchPattern(p, s) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, s string) bool {
	parts := strings.Split(pattern, "%")
	if len(parts) == 1 {
		return pattern == s
	}

	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]

	for _, middle := range parts[1 : len(parts)-1] {
		i := strings.Index(s, middle)
		if i < 0 {
			return false
		}
		s = s[i+len(middle):]
	}

	return strings.HasSuffix(s, parts[len(parts)-1])
}
