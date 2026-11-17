package collector

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestRedactExplainPlanJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "keeps numbers, booleans, and null exactly as returned",
			input: `[{"Plan":{"Node Type":"Seq Scan","Startup Cost":0.00,"Total Cost":155.00,"Plan Rows":10000,"Plan Width":36,"Parallel Aware":false,"Extra":null}}]`,
			want:  `[{"Plan":{"Node Type":"Seq Scan","Startup Cost":0.00,"Total Cost":155.00,"Plan Rows":10000,"Plan Width":36,"Parallel Aware":false,"Extra":null}}]`,
		},
		{
			name:  "redacts literals in a filter and keeps its structure",
			input: `[{"Plan":{"Node Type":"Seq Scan","Filter":"((status = 'open'::text) AND (amount > 100) AND (a < b))"}}]`,
			want:  `[{"Plan":{"Node Type":"Seq Scan","Filter":"((status = ?::text) AND (amount > ?) AND (a < b))"}}]`,
		},
		{
			name:  "redacts every string of an expression array",
			input: `[{"Plan":{"Output":["o.id","'secret'::text","(o.amount * 2)"]}}]`,
			want:  `[{"Plan":{"Output":["o.id","?::text","(o.amount * ?)"]}}]`,
		},
		{
			name:  "redacts nested arrays under an expression key",
			input: `[{"Plan":{"Group Keys":[["a","'x'::text"],["b"]]}}]`,
			want:  `[{"Plan":{"Group Keys":[["a","?::text"],["b"]]}}]`,
		},
		{
			name:  "keeps parameter placeholders",
			input: `[{"Plan":{"Index Cond":"(id = $1)"}}]`,
			want:  `[{"Plan":{"Index Cond":"(id = $1)"}}]`,
		},
		{
			name:  "redacts SQL under a key it does not know",
			input: `[{"Plan":{"Remote SQL":"SELECT id FROM t WHERE x = 'abc' AND y > 5"}}]`,
			want:  `[{"Plan":{"Remote SQL":"SELECT id FROM t WHERE x = ? AND y > ?"}}]`,
		},
		{
			name:  "keeps plain words under a key it does not know",
			input: `[{"Plan":{"Some New Label":"Seq Scan 2"}}]`,
			want:  `[{"Plan":{"Some New Label":"Seq Scan 2"}}]`,
		},
		{
			name: "keeps names and labels",
			input: `[{"Plan":{"Node Type":"Index Scan","Parent Relationship":"SubPlan","Subplan Name":"SubPlan 2","CTE Name":"My CTE",` +
				`"Relation Name":"orders","Schema":"public","Alias":"o","Scan Direction":"Backward","Join Type":"Inner"}}]`,
			want: `[{"Plan":{"Node Type":"Index Scan","Parent Relationship":"SubPlan","Subplan Name":"SubPlan 2","CTE Name":"My CTE",` +
				`"Relation Name":"orders","Schema":"public","Alias":"o","Scan Direction":"Backward","Join Type":"Inner"}}]`,
		},
		{
			name:  "keeps identifiers with hyphens, quotes, and non-ASCII letters",
			input: `[{"Plan":{"Index Name":"idx_orders_cust-id","Relation Name":"\"Weird Table\"","Alias":"café"}}]`,
			want:  `[{"Plan":{"Index Name":"idx_orders_cust-id","Relation Name":"\"Weird Table\"","Alias":"café"}}]`,
		},
		{
			name: "keeps names that start with a digit",
			input: `[{"Plan":{"Relation Name":"1tbl","Alias":"2024_orders","Index Name":"3idx","Schema":"9schema","CTE Name":"1cte",` +
				`"Subplan Name":"1sub","Filter":"((\"1tbl\".id = 5) AND (\"2024_orders\".x = 'a'::text))"}}]`,
			want: `[{"Plan":{"Relation Name":"1tbl","Alias":"2024_orders","Index Name":"3idx","Schema":"9schema","CTE Name":"1cte",` +
				`"Subplan Name":"1sub","Filter":"((\"1tbl\".id = ?) AND (\"2024_orders\".x = ?::text))"}}]`,
		},
		{
			name:  "still redacts a digit-leading value under an unknown key",
			input: `[{"Plan":{"Some New Label":"1tbl"}}]`,
			want:  `[{"Plan":{"Some New Label":"?tbl"}}]`,
		},
		{
			name:  "does not escape HTML characters",
			input: `[{"Plan":{"Filter":"((a < b) AND (c > d) AND (e & f))"}}]`,
			want:  `[{"Plan":{"Filter":"((a < b) AND (c > d) AND (e & f))"}}]`,
		},
		{
			name:  "keeps the order of keys and array elements",
			input: `[{"Plan":{"Z":1,"A":2,"Plans":[{"Node Type":"Second"},{"Node Type":"First"}]}}]`,
			want:  `[{"Plan":{"Z":1,"A":2,"Plans":[{"Node Type":"Second"},{"Node Type":"First"}]}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := redactExplainPlanJSON([]byte(tt.input))
			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
			require.True(t, json.Valid(got))
		})
	}
}

func TestRedactExplainPlanJSON_ExpressionKeys(t *testing.T) {
	keys := make([]string, 0, len(explainPlanExpressionKeys))
	for key := range explainPlanExpressionKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			scalar := `{"` + key + `":"(secret_col = 'hunter2'::text AND other = 4242)"}`
			got, err := redactExplainPlanJSON([]byte(scalar))
			require.NoError(t, err)
			require.NotContains(t, string(got), "hunter2")
			require.NotContains(t, string(got), "4242")
			require.Contains(t, string(got), "secret_col", "column names are not query data")

			list := `{"` + key + `":["(secret_col = 'hunter2'::text AND other = 4242)"]}`
			got, err = redactExplainPlanJSON([]byte(list))
			require.NoError(t, err)
			require.NotContains(t, string(got), "hunter2")
			require.NotContains(t, string(got), "4242")
		})
	}
}

func TestRedactExplainPlanJSON_Invalid(t *testing.T) {
	for name, input := range map[string]string{
		"empty":         ``,
		"not json":      `not json`,
		"truncated":     `[{"Plan":`,
		"unclosed":      `[{"Plan":{"Node Type":"Seq Scan"}}`,
		"trailing data": `[{"Plan":{}}] [{"Plan":{}}]`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := redactExplainPlanJSON([]byte(input))
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
}

// TestRedactExplainPlanJSON_Fixtures runs the redaction over plans captured
// from PostgreSQL and checks that it changes only SQL expressions.
func TestRedactExplainPlanJSON_Fixtures(t *testing.T) {
	// Keys whose values must come through exactly as PostgreSQL returned them.
	identifierKeys := map[string]struct{}{
		"Node Type": {}, "Parent Relationship": {}, "Subplan Name": {}, "Join Type": {},
		"Scan Direction": {}, "Strategy": {}, "Partial Mode": {}, "Relation Name": {},
		"Schema": {}, "Alias": {}, "Index Name": {}, "CTE Name": {},
	}

	files, err := filepath.Glob("./testdata/explain_plan/*.txtar")
	require.NoError(t, err)
	require.NotEmpty(t, files)

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			archive, err := txtar.ParseFile(file)
			require.NoError(t, err)
			require.Len(t, archive.Files, 1)
			plan := archive.Files[0].Data

			got, err := redactExplainPlanJSON(plan)
			require.NoError(t, err)
			require.True(t, json.Valid(got))

			var before, after any
			decode := func(data []byte, v *any) {
				dec := json.NewDecoder(bytes.NewReader(data))
				dec.UseNumber()
				require.NoError(t, dec.Decode(v))
			}
			decode(plan, &before)
			decode(got, &after)

			assertOnlyExpressionsRedacted(t, "", before, after, identifierKeys)
		})
	}
}

func assertOnlyExpressionsRedacted(t *testing.T, key string, before, after any, identifierKeys map[string]struct{}) {
	t.Helper()

	switch b := before.(type) {
	case map[string]any:
		a, ok := after.(map[string]any)
		require.True(t, ok, "object became %T", after)
		require.Len(t, a, len(b))
		for k, bv := range b {
			av, ok := a[k]
			require.True(t, ok, "key %q was removed", k)
			assertOnlyExpressionsRedacted(t, k, bv, av, identifierKeys)
		}
	case []any:
		a, ok := after.([]any)
		require.True(t, ok, "array became %T", after)
		require.Len(t, a, len(b))
		for i := range b {
			assertOnlyExpressionsRedacted(t, key, b[i], a[i], identifierKeys)
		}
	case string:
		a, ok := after.(string)
		require.True(t, ok, "string under %q became %T", key, after)
		if _, ok := identifierKeys[key]; ok {
			require.Equal(t, b, a, "value of %q must not change", key)
		}
		require.False(t, strings.Contains(a, "'"), "value of %q still has a quoted literal: %s", key, a)
		if _, ok := explainPlanExpressionKeys[key]; ok {
			require.Equal(t, database_observability.RedactSql(b), a, "value of %q must be redacted as SQL", key)
		}
	default:
		// Numbers and booleans.
		require.Equal(t, before, after, "value of %q must not change", key)
	}
}
