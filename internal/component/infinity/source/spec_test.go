package source

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPostProcessErrorText locks the message that each post-processing step
// gives. The index of an explicit transform counts only the transform list.
// The library text can quote the response, so only the detail has it.
func TestPostProcessErrorText(t *testing.T) {
	tests := map[string]struct{ cfg, step, detail string }{
		"computed column": {`
			computed_column {
				selector = "a +"
				text     = "b"
			}`, "computed columns", "unexpected end of expression"},
		"filter_expression": {`
			filter_expression = "a >"`, "filter_expression", "unexpected end of expression"},
		"summarize": {`
			summarize_expression = "nope(a)"`, "summarize", "error evaluating summarize expression. Undefined function nope. Not applying summarize expression"},
		"transform computed_column": {`
			transform.limit {
				limit = 5
			}
			transform.computed_column {
				expression = "a +"
				alias      = "b"
			}`, "transform 1 (computed_column)", "unexpected end of expression"},
		"transform filter": {`
			filter_expression = "a > 0"
			transform.filter {
				expression = "a >"
			}`, "transform 0 (filter)", "unexpected end of expression"},
		"transform summarize": {`
			transform.summarize {
				expression = "nope(a)"
			}`, "transform 0 (summarize)", "error evaluating summarize expression. Undefined function nope. Not applying summarize expression"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s, err := specFromConfig(`query "q" {
				url = "http://x"` + tt.cfg + `
			}`)
			require.NoError(t, err)
			_, err = buildFrame(s, []byte(`[{"a":1},{"a":2}]`))
			require.Equal(t, reasonPostprocess, reasonOf(err))
			require.Equal(t, tt.step+" failed", err.Error())
			var pe *pollError
			require.ErrorAs(t, err, &pe)
			require.Equal(t, tt.detail, pe.detail())
		})
	}
}

// TestComputedColumnsSeeEarlierColumns checks that each computed column can
// use the computed columns before it.
func TestComputedColumnsSeeEarlierColumns(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://x"
		computed_column {
			selector = "a * 10"
			text     = "b"
		}
		computed_column {
			selector = "b + 1"
			text     = "c"
		}
	}`)
	require.NoError(t, err)
	f, err := buildFrame(s, []byte(`[{"a":1},{"a":2}]`))
	require.NoError(t, err)
	c, ok := fieldValues(f, "c")
	require.True(t, ok)
	require.Equal(t, []any{11.0, 21.0}, c)
}

// TestSummarizeAliasDefaults locks the two defaults. summarize_alias
// defaults to "summary", as in the plugin. The alias of a summarize
// transform defaults to its expression, as in the library.
func TestSummarizeAliasDefaults(t *testing.T) {
	tests := map[string]struct{ cfg, want string }{
		"summarize_expression": {`summarize_expression = "count(a)"`, "summary"},
		"transform": {`transform.summarize {
			expression = "count(a)"
		}`, "count(a)"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s, err := specFromConfig(`query "q" {
				url = "http://x"
				` + tt.cfg + `
			}`)
			require.NoError(t, err)
			f, err := buildFrame(s, []byte(`[{"a":1},{"a":2}]`))
			require.NoError(t, err)
			v, ok := fieldValues(f, tt.want)
			require.True(t, ok)
			require.Equal(t, []any{2.0}, v)
		})
	}
}
