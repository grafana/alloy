package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

// tableNameFixture holds an explain plan and its rewritten query note, as
// MySQL returned them for the same query.
type tableNameFixture struct {
	plan []byte
	note string
}

// loadTableNameFixture reads a plan and its note from a fixture captured from
// real servers.
func loadTableNameFixture(t *testing.T, name string) tableNameFixture {
	t.Helper()
	archive, err := txtar.ParseFile("./testdata/explain_plan_table_names/" + name + ".txtar")
	require.NoError(t, err)

	var fixture tableNameFixture
	for _, f := range archive.Files {
		switch f.Name {
		case "plan.json":
			fixture.plan = f.Data
		case "note":
			fixture.note = strings.TrimSpace(string(f.Data))
		}
	}
	require.NotEmpty(t, fixture.plan, name)
	require.NotEmpty(t, fixture.note, name)
	return fixture
}

// resolvePlanTables resolves every table access in a plan, as "select:name"
// mapped to "schema.table", "derived" or "unknown".
func resolvePlanTables(t *testing.T, plan []byte, names planTableNames) map[string]string {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(plan))
	decoder.UseNumber()
	var root any
	require.NoError(t, decoder.Decode(&root))

	got := map[string]string{}
	var walk func(node any, selectID int)
	walk = func(node any, selectID int) {
		switch n := node.(type) {
		case map[string]any:
			if id, ok := n["select_id"].(json.Number); ok {
				v, err := id.Int64()
				require.NoError(t, err)
				selectID = int(v)
			}
			if name, ok := n["table_name"].(string); ok && !strings.HasPrefix(name, "<") {
				if _, ok := n["access_type"]; ok {
					key := fmt.Sprintf("%d:%s", selectID, name)
					switch ref, kind := names.resolve(selectID, name); kind {
					case planTableReal:
						got[key] = ref.schema + "." + ref.table
					case planTableDerived:
						got[key] = "derived"
					default:
						got[key] = "unknown"
					}
				}
			}
			for _, child := range n {
				walk(child, selectID)
			}
		case []any:
			for _, child := range n {
				walk(child, selectID)
			}
		}
	}
	walk(root, 0)
	return got
}

func TestParseRewrittenQueryTablesFromRealServers(t *testing.T) {
	for name, want := range map[string]map[string]string{
		"aliased_join": {"1:o": "shop.orders", "1:c": "shop.customers"},
		"unaliased":    {"1:orders": "shop.orders"},
		// The same alias names a different table in each query block.
		"alias_reused_in_scalar_subquery": {"1:o": "shop.orders", "2:o": "shop.customers"},
		// A semijoin merges both blocks, leaving two tables named o in one.
		"same_alias_merged_by_semijoin": {"1:o": "unknown"},
		"cross_schema":                  {"1:e": "archive.events"},
		"derived_table":                 {"1:c": "shop.customers", "1:t": "derived", "2:orders": "shop.orders"},
		// A merged view shows up as its base table, without the view's alias.
		"merged_view": {"1:customers": "shop.customers"},
		"union":       {"1:orders": "shop.orders", "2:events": "archive.events"},
		// Table-like text inside a string literal is not a table.
		"literal_that_looks_like_a_table": {"1:o": "shop.orders"},
		"cte":                             {"1:orders": "shop.orders", "1:c": "shop.customers"},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := loadTableNameFixture(t, name)
			names := parseRewrittenQueryTables(fixture.note)
			require.Equal(t, want, resolvePlanTables(t, fixture.plan, names))
		})
	}
}

func TestParseRewrittenQueryTables(t *testing.T) {
	for name, tc := range map[string]struct {
		note string
		want map[string]string
	}{
		"identifiers with escaped backticks": {
			note: "/* select#1 */ select `s``1`.`t``1`.`id` AS `id` from `s``1`.`t``1` `a``b`",
			want: map[string]string{"1:a`b": "s`1.t`1"},
		},
		"strings with escaped quotes": {
			note: "/* select#1 */ select `s`.`t`.`id` AS `id` from `s`.`t` where (`s`.`t`.`n` = 'it''s from `x`.`y` \\' join `p`.`q`')",
			want: map[string]string{"1:t": "s.t"},
		},
		"comma joins": {
			note: "/* select#1 */ select 1 AS `1` from `s`.`a` `x`, `s`.`b` `y` where (`s`.`x`.`id` = `s`.`y`.`id`)",
			want: map[string]string{"1:x": "s.a", "1:y": "s.b"},
		},
		"commas inside a join condition don't start tables": {
			note: "/* select#1 */ select 1 AS `1` from `s`.`a` `x` left join `s`.`b` `y` on((`s`.`y`.`id` = ifnull(`s`.`x`.`id`,`t`.`z`))) where true",
			want: map[string]string{"1:x": "s.a", "1:y": "s.b"},
		},
		"straight join": {
			note: "/* select#1 */ select 1 AS `1` from `s`.`a` straight_join `s`.`b`",
			want: map[string]string{"1:a": "s.a", "1:b": "s.b"},
		},
		"no note": {
			note: "",
			want: map[string]string{},
		},
		"unterminated comment": {
			note: "/* select#1 */ select 1 from `s`.`a` /* never closed",
			want: map[string]string{"1:a": "s.a"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			names := parseRewrittenQueryTables(tc.note)
			got := map[string]string{}
			for k, ref := range names.refs {
				got[fmt.Sprintf("%d:%s", k.selectID, k.name)] = ref.schema + "." + ref.table
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestPlanTableNamesResolve(t *testing.T) {
	names := parseRewrittenQueryTables("/* select#1 */ select 1 from `s`.`a` `x` join (/* select#2 */ select 1 from `s`.`b`) `d` where (`s`.`x`.`id` in (/* select#3 */ select 1 from `s`.`c` `y`))")

	ref, kind := names.resolve(1, "x")
	require.Equal(t, planTableReal, kind)
	require.Equal(t, tableRef{schema: "s", table: "a"}, ref)

	_, kind = names.resolve(1, "d")
	require.Equal(t, planTableDerived, kind)

	// A name that is unique across the query resolves from any query block.
	ref, kind = names.resolve(7, "y")
	require.Equal(t, planTableReal, kind)
	require.Equal(t, tableRef{schema: "s", table: "c"}, ref)

	_, kind = names.resolve(1, "missing")
	require.Equal(t, planTableUnknown, kind)

	// Nothing resolves without a rewritten query.
	_, kind = planTableNames{}.resolve(1, "x")
	require.Equal(t, planTableUnknown, kind)
}
