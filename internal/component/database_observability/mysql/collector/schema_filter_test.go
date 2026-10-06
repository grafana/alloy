package collector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaFilter(t *testing.T) {
	for name, tc := range map[string]struct {
		filter     SchemaFilter
		restricted bool
		selected   []string
		rejected   []string
	}{
		"zero value selects everything": {
			filter:   SchemaFilter{},
			selected: []string{"app", "hg_a", ""},
		},
		"exact include": {
			filter:     SchemaFilter{Include: []string{"app"}},
			restricted: true,
			selected:   []string{"app"},
			rejected:   []string{"app2", "xapp", "App"},
		},
		"prefix pattern": {
			filter:     SchemaFilter{Include: []string{"hg_%"}},
			restricted: true,
			selected:   []string{"hg_a", "hg_", "hg_a_b"},
			rejected:   []string{"hgwarm_1", "hg", "xhg_a"},
		},
		"underscore is not a wildcard": {
			filter:     SchemaFilter{Include: []string{"hg_%"}},
			restricted: true,
			rejected:   []string{"hgwarm_1", "hgx1"},
		},
		"suffix and middle patterns": {
			filter:     SchemaFilter{Include: []string{"%_test", "a%b%c"}},
			restricted: true,
			selected:   []string{"x_test", "_test", "abc", "aXbYc", "abbc"},
			rejected:   []string{"test", "ab", "acb", "a_test_x"},
		},
		"percent alone selects everything": {
			filter:     SchemaFilter{Include: []string{"%"}},
			restricted: true,
			selected:   []string{"anything", ""},
		},
		"exclude only": {
			filter:     SchemaFilter{Exclude: []string{"hgwarm_%", "scratch"}},
			restricted: true,
			selected:   []string{"app", "hg_a", "scratch2"},
			rejected:   []string{"hgwarm_1", "scratch"},
		},
		"exclude wins over include": {
			filter:     SchemaFilter{Include: []string{"hg%"}, Exclude: []string{"hgwarm_%"}},
			restricted: true,
			selected:   []string{"hg_a", "hgx"},
			rejected:   []string{"hgwarm_1", "app"},
		},
		"overlapping parts of a pattern do not match twice": {
			filter:     SchemaFilter{Include: []string{"a%a"}},
			restricted: true,
			selected:   []string{"aa", "aXa"},
			rejected:   []string{"a"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.restricted, tc.filter.Restricted())
			for _, s := range tc.selected {
				require.True(t, tc.filter.Match(s), "%q should be selected", s)
			}
			for _, s := range tc.rejected {
				require.False(t, tc.filter.Match(s), "%q should not be selected", s)
			}
		})
	}
}
