package sdlc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiffObjectsModes(t *testing.T) {
	rule := &diffRule{fields: map[string]*diffRule{
		"value":   {mode: diffValue},
		"op":      {mode: diffOperation},
		"ignored": {mode: diffIgnore},
		"items":   {keyBy: "name", order: "items.order", each: &diffRule{fields: map[string]*diffRule{"name": {mode: diffIgnore}, "v": {mode: diffValue}}}},
		"vars":    {keyBy: "name", each: &diffRule{mode: diffOperation}},
		"tags":    {each: &diffRule{mode: diffValue}},
	}}
	before := map[string]any{
		"value": "a", "op": "secret-old", "ignored": "x", "unknown": map[string]any{"token": "secret-1"},
		"items": []any{map[string]any{"name": "one", "v": "1"}, map[string]any{"name": "two", "v": "2"}},
		"vars":  []any{map[string]any{"name": "KEEP", "value": "k"}, map[string]any{"name": "DROP", "value": "secret-d"}},
		"tags":  map[string]any{"a": "1"},
	}
	after := map[string]any{
		"value": "b", "op": "secret-new", "ignored": "y", "unknown": map[string]any{"token": "secret-2"},
		"items": []any{map[string]any{"name": "one", "v": "1b"}, map[string]any{"name": "three", "v": "3"}},
		"vars":  []any{map[string]any{"name": "KEEP", "value": "k"}, map[string]any{"name": "ADD", "value": "secret-a"}},
		"tags":  map[string]any{"b": "2"},
	}
	s := func(v string) *string { return &v }
	require.Equal(t, []fieldChange{
		{Path: "items.one.v", Operation: operationModified, Before: s("1"), After: s("1b")},
		{Path: "items.three", Operation: operationAdded},
		{Path: "items.three.v", Operation: operationAdded, After: s("3")},
		{Path: "items.two", Operation: operationRemoved},
		{Path: "items.two.v", Operation: operationRemoved, Before: s("2")},
		{Path: "op", Operation: operationModified},
		{Path: "tags.a", Operation: operationRemoved, Before: s("1")},
		{Path: "tags.b", Operation: operationAdded, After: s("2")},
		// Fields without a rule are reported by name only.
		{Path: "unknown", Operation: operationModified},
		{Path: "value", Operation: operationModified, Before: s("a"), After: s("b")},
		{Path: "vars.ADD", Operation: operationAdded},
		{Path: "vars.DROP", Operation: operationRemoved},
	}, diffObjects(rule, before, after))
	require.Empty(t, diffObjects(rule, before, before))

	reordered := map[string]any{"items": []any{map[string]any{"name": "two", "v": "2"}, map[string]any{"name": "one", "v": "1"}}}
	original := map[string]any{"items": before["items"]}
	require.Equal(t, []fieldChange{{Path: "items.order", Operation: operationModified}}, diffObjects(rule, original, reordered))
}

func TestDiffObjectsEmptyEqualsMissing(t *testing.T) {
	rule := &diffRule{fields: map[string]*diffRule{"tags": {each: &diffRule{mode: diffValue}}}}
	require.Empty(t, diffObjects(rule, map[string]any{}, map[string]any{"tags": map[string]any{}, "list": []any{}, "s": ""}))
	require.Empty(t, diffObjects(rule, nil, map[string]any{}))
}
