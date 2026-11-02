package database_observability

import "strings"

// MergeExclusions merges exclusion lists in order and removes duplicates.
func MergeExclusions(exclusionLists ...[]string) []string {
	seen := make(map[string]struct{})
	var merged []string
	for _, exclusions := range exclusionLists {
		for _, exclusion := range exclusions {
			if _, exists := seen[exclusion]; exists {
				continue
			}
			seen[exclusion] = struct{}{}
			merged = append(merged, exclusion)
		}
	}
	return merged
}

// ResolveExclusions merges mandatory exclusions with the override when it is set,
// otherwise with the inherited exclusions.
func ResolveExclusions(mandatory, inherited, override []string) []string {
	effective := inherited
	if override != nil {
		effective = override
	}
	return MergeExclusions(mandatory, effective)
}

// BuildExclusionClause builds a SQL IN clause from a list of items.
func BuildExclusionClause(items []string) string {
	escaped := make([]string, len(items))
	for i, item := range items {
		escaped[i] = escapeSQLString(item)
	}
	return "(" + strings.Join(escaped, ", ") + ")"
}

// escapeSQLString escapes single quotes by doubling them to prevent SQL injection.
func escapeSQLString(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}
