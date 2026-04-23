// Package template holds the naive ${NAME} -> value substitution used to
// inject per-test runtime identity into workload YAML and Helm values files.
//
// This is intentionally minimal:
//
//   - unknown placeholders (e.g. ${UNKNOWN}) pass through verbatim rather
//     than raising an error so new placeholders can be added without a
//     forced migration of all test assets;
//   - substitution is deterministic: keys are replaced in sorted order;
//   - no escaping, no expressions, no conditionals.
//
// For richer templating, reach for text/template rather than bolting onto
// this helper.
package template

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// RenderFile reads path, substitutes ${KEY} placeholders with the matching
// map value, and writes the result to a new temp file. It returns the temp
// file path; callers are responsible for removing it (via os.Remove).
func RenderFile(path string, vars map[string]string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	content := Render(string(raw), vars)

	tmp, err := os.CreateTemp("", "k8s-v2-rendered-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create rendered temp file: %w", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write rendered temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close rendered temp file: %w", err)
	}
	return tmp.Name(), nil
}

// Render substitutes ${KEY} placeholders in content with the matching map
// value. Unknown placeholders pass through unchanged (see package doc).
func Render(content string, vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		content = strings.ReplaceAll(content, "${"+key+"}", vars[key])
	}
	return content
}
