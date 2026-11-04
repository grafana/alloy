package database_observability

import (
	"fmt"
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

// ResolveExclusions merges default exclusions with the override when it is set,
// otherwise with the inherited exclusions.
func ResolveExclusions(defaults, inherited, override []string) []string {
	effective := inherited
	if override != nil {
		effective = override
	}
	return MergeExclusions(defaults, effective)
}

// UserConfiguredExclusions returns nil when configured equals defaults,
// and returns configured otherwise.
func UserConfiguredExclusions(configured, defaults []string) []string {
	if slices.Equal(configured, defaults) {
		return nil
	}
	return configured
}

// EffectiveDefaultExclusions returns the items in defaults that are not present
// in include.
func EffectiveDefaultExclusions(defaults, include []string, caseInsensitive bool) []string {
	if len(include) == 0 {
		return MergeExclusions(defaults)
	}
	includedSet := make(map[string]struct{}, len(include))
	for _, item := range include {
		key := item
		if caseInsensitive {
			key = strings.ToLower(item)
		}
		includedSet[key] = struct{}{}
	}
	var filtered []string
	for _, item := range defaults {
		key := item
		if caseInsensitive {
			key = strings.ToLower(item)
		}
		if _, exists := includedSet[key]; !exists {
			filtered = append(filtered, item)
		}
	}
	return MergeExclusions(filtered)
}

// EffectiveExclusions merges the default exclusions (minus any items explicitly
// included) with the user-configured exclusions.
func EffectiveExclusions(defaults, configuredExclude, configuredInclude []string, caseInsensitive bool) []string {
	return MergeExclusions(
		EffectiveDefaultExclusions(defaults, configuredInclude, caseInsensitive),
		UserConfiguredExclusions(configuredExclude, defaults),
	)
}

// ResolveInclusions returns the override inclusion list when it is set,
// otherwise the inherited inclusions, removing duplicates while preserving order.
func ResolveInclusions(inherited, override []string) []string {
	effective := inherited
	if override != nil {
		effective = override
	}
	return MergeExclusions(effective)
}

// ValidateInclusions returns an error if include overlaps the effective exclusions.
func ValidateInclusions(include, alwaysExcluded, configuredExclude, defaults []string, includeAttr, excludeAttr string, caseInsensitive bool) error {
	exclude := MergeExclusions(alwaysExcluded, UserConfiguredExclusions(configuredExclude, defaults))
	return validateDisjointLists(include, exclude, includeAttr, excludeAttr, caseInsensitive)
}

// validateDisjointLists returns an error if any item appears in both include and exclude.
func validateDisjointLists(include, exclude []string, includeAttr, excludeAttr string, caseInsensitive bool) error {
	if len(include) == 0 || len(exclude) == 0 {
		return nil
	}
	excludedSet := make(map[string]struct{}, len(exclude))
	for _, item := range exclude {
		key := item
		if caseInsensitive {
			key = strings.ToLower(item)
		}
		excludedSet[key] = struct{}{}
	}
	for _, item := range include {
		key := item
		if caseInsensitive {
			key = strings.ToLower(item)
		}
		if _, exists := excludedSet[key]; exists {
			return fmt.Errorf("%s and %s cannot both contain %q", includeAttr, excludeAttr, item)
		}
	}
	return nil
}

// SameExclusions reports whether two exclusion lists contain the same set of
// items, regardless of order or duplicates.
func SameExclusions(a, b []string) bool {
	normA := MergeExclusions(a)
	normB := MergeExclusions(b)
	slices.Sort(normA)
	slices.Sort(normB)
	return slices.Equal(normA, normB)
}

// BuildExclusionClause builds a SQL IN clause from a list of items.
func BuildExclusionClause(items []string) string {
	escaped := make([]string, len(items))
	for i, item := range items {
		escaped[i] = escapeSQLString(item)
	}
	return "(" + strings.Join(escaped, ", ") + ")"
}

// BuildInclusionClause builds a SQL "AND <column> IN" clause from a list of items.
func BuildInclusionClause(columnExpr string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	return fmt.Sprintf(" AND %s IN %s", columnExpr, BuildExclusionClause(items))
}

// escapeSQLString escapes single quotes by doubling them to prevent SQL injection.
func escapeSQLString(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}
