// Package template holds the ${NAME} -> value substitution used to inject
// per-test runtime identity into workload YAML and Helm values files.
//
// Design notes:
//
//   - Substitution walks the input once via os.Expand, so a var's value is
//     used literally even if it happens to contain another placeholder. No
//     recursive expansion is performed.
//   - Unknown placeholders (e.g. ${UNKNOWN}) pass through verbatim rather
//     than erroring so new placeholders can be added without a forced
//     migration of every test asset.
//   - Only brace form ${KEY} is substituted. Bare $KEY is left untouched to
//     avoid surprising substitutions in YAML with shell-like strings.
//
// For richer templating, reach for text/template rather than extending this
// helper.
package template

import (
	"fmt"
	"os"
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
// value. Unknown placeholders pass through unchanged. Only brace form is
// recognised; bare $KEY is left literal.
func Render(content string, vars map[string]string) string {
	// Re-insert $ where os.Expand saw a bare $KEY we don't want to expand.
	// We accomplish that by only substituting when the original source had
	// the `${KEY}` form: os.Expand always strips the $ and (optional) braces,
	// so we detect the bare form via a sentinel wrap and restore it.
	const sentinel = "\x00k8sv2-bare-dollar\x00"
	// Replace any bare `$` (not followed by `{`) with a sentinel so os.Expand
	// doesn't see it as a variable prefix.
	protected := protectBareDollars(content, sentinel)
	expanded := os.Expand(protected, func(key string) string {
		if v, ok := vars[key]; ok {
			return v
		}
		return "${" + key + "}"
	})
	return strings.ReplaceAll(expanded, sentinel, "$")
}

// protectBareDollars replaces every `$` that is NOT followed by `{` with
// sentinel, so a subsequent os.Expand pass only expands the brace form.
func protectBareDollars(s, sentinel string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && (i+1 >= len(s) || s[i+1] != '{') {
			b.WriteString(sentinel)
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
