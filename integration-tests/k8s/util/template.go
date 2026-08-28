// Package util provides helpers shared across runner / harness / deps.
package util

import (
	"fmt"
	"regexp"
	"sort"
)

// placeholderRE matches ${UPPER_CASE_NAME}. Bare $FOO is not matched so the
// substitution is safe on YAML/JSON containing unrelated dollar signs.
var placeholderRE = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

// SubstituteVars replaces ${KEY} with vars[KEY]. Returns an error listing
// any unresolved keys (deduplicated, sorted) so typos fail loudly.
func SubstituteVars(content string, vars map[string]string) (string, error) {
	var unresolved []string
	out := placeholderRE.ReplaceAllStringFunc(content, func(match string) string {
		key := match[2 : len(match)-1]
		if v, ok := vars[key]; ok {
			return v
		}
		unresolved = append(unresolved, key)
		return match
	})
	if len(unresolved) > 0 {
		seen := make(map[string]struct{}, len(unresolved))
		unique := unresolved[:0]
		for _, k := range unresolved {
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			unique = append(unique, k)
		}
		sort.Strings(unique)
		return "", fmt.Errorf("unresolved ${VAR} placeholders: %v", unique)
	}
	return out, nil
}
