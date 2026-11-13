package sdlc

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var placeholderPattern = "^" + regexp.QuoteMeta(redactedMarker) + "[0-9a-f]{8}]$"

// requirePlaceholder asserts that value is a redacted placeholder.
func requirePlaceholder(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	require.Regexp(t, placeholderPattern, value, msgAndArgs...)
}

func TestRedactedPlaceholders(t *testing.T) {
	requirePlaceholder(t, redacted("secret-env"))
	// Equal values share a placeholder and different values don't, without
	// revealing either.
	require.Equal(t, redacted("secret-env"), redacted("secret-env"))
	require.NotEqual(t, redacted("secret-env"), redacted("secret-env-2"))
	require.NotEqual(t, redacted("1"), redacted(int64(1)), "a string differs from a number")
	require.NotContains(t, redacted("secret-env"), "secret")
}

func TestSnapshotModes(t *testing.T) {
	rule := &snapshotRule{fields: map[string]*snapshotRule{
		"kept":    keepRule,
		"secret":  redactRule,
		"omitted": omitRule,
		"url":     redactQueryRule,
		"plain":   redactQueryRule,
		"items":   {items: &snapshotRule{fields: map[string]*snapshotRule{"value": redactRule}}},
		"tags":    {each: redactRule},
	}}
	got := snapshotNode(rule, map[string]any{
		"kept": "a", "secret": []any{"x", map[string]any{"k": "y"}}, "omitted": "z",
		"url": "/ready?token=t", "plain": "/ready", "empty": "", "unknown": map[string]any{"n": int64(1)},
		// A list element that isn't an object keeps only its shape.
		"items": []any{map[string]any{"name": "one", "value": "v"}, "bare"},
		"tags":  map[string]any{"a": "1"},
	})
	require.Equal(t, map[string]any{
		"kept":    "a",
		"secret":  []any{redacted("x"), map[string]any{"k": redacted("y")}},
		"url":     "/ready?" + redacted("token=t"),
		"plain":   "/ready",
		"unknown": map[string]any{"n": int64(1)},
		"items":   []any{map[string]any{"name": "one", "value": redacted("v")}, redacted("bare")},
		"tags":    map[string]any{"a": redacted("1")},
	}, got)
	require.Nil(t, snapshotNode(rule, map[string]any{"omitted": "z", "tags": map[string]any{}}))
}

func TestSameState(t *testing.T) {
	a := map[string]any{"metadata": map[string]any{"name": "app", "resourceVersion": "1"}, "data": map[string]any{"k": "v"}}
	b := map[string]any{"metadata": map[string]any{"name": "app", "resourceVersion": "2"}, "data": map[string]any{"k": "v"}}
	require.True(t, sameState(a, b), "the resource version changes with every write")
	require.Equal(t, "1", a["metadata"].(map[string]any)["resourceVersion"], "snapshots aren't modified")
	b["data"] = map[string]any{"k": "w"}
	require.False(t, sameState(a, b))
}
