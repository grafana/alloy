package collector

import "strings"

// SchemaFilter selects schemas by name. A schema is selected when Include is
// empty or the schema matches one of its patterns.
//
// A pattern is matched against the whole schema name. '%' matches any run of
// characters, including none; every other character, '_' included, matches
// itself, so "hg_%" does not match "hgwarm_1".
//
// There is deliberately no exclude list: performance_schema's index only helps
// a query that names the schemas it wants, and a NOT IN list can't use it.
type SchemaFilter struct {
	Include []string
}

// Restricted reports whether the filter can leave out any schema.
func (f SchemaFilter) Restricted() bool {
	return len(f.Include) > 0
}

// Match reports whether schema is selected.
func (f SchemaFilter) Match(schema string) bool {
	return len(f.Include) == 0 || matchesAny(f.Include, schema)
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
