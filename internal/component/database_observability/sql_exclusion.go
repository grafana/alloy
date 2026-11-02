package database_observability

import (
	"slices"
	"strings"
)

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

// ResolveExclusionOverride returns the override when it is set, otherwise it returns the inherited exclusions.
func ResolveExclusionOverride(inherited, override []string) []string {
	if override != nil {
		return slices.Clone(override)
	}
	return slices.Clone(inherited)
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
