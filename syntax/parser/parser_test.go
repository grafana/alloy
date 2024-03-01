package parser

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParser(f *testing.F) {
	filepath.WalkDir("./testdata/valid", func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}

		bb, err := os.ReadFile(path)
		require.NoError(f, err)
		f.Add(bb)
		return nil
	})

	f.Fuzz(func(t *testing.T, input []byte) {
		p := newParser(t.Name(), input)

		_ = p.ParseFile()
		if len(p.diags) > 0 {
			t.SkipNow()
		}
	})
}

// TestValid parses every *.river file in testdata, which is expected to be
// valid.
func TestValid(t *testing.T) {
	filepath.WalkDir("./testdata/valid", func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}

		t.Run(filepath.Base(path), func(t *testing.T) {
			bb, err := os.ReadFile(path)
			require.NoError(t, err)

			p := newParser(path, bb)

			res := p.ParseFile()
			require.NotNil(t, res)
			require.Len(t, p.diags, 0)
		})

		return nil
	})
}

func TestParseExpressions(t *testing.T) {
	tt := map[string]string{
		"literal number": `10`,
		"literal float":  `15.0`,
		"literal string": `"Hello, world!"`,
		"literal ident":  `some_ident`,
		"literal null":   `null`,
		"literal true":   `true`,
		"literal false":  `false`,

		"empty array":          `[]`,
		"array one element":    `[1]`,
		"array many elements":  `[0, 1, 2, 3]`,
		"array trailing comma": `[0, 1, 2, 3,]`,
		"nested array":         `[[0, 1, 2], [3, 4, 5]]`,
		"array multiline": `[
			0,
			1, 
			2,
		]`,

		"empty object":           `{}`,
		"object one field":       `{ field_a = 5 }`,
		"object multiple fields": `{ field_a = 5, field_b = 10 }`,
		"object trailing comma":  `{ field_a = 5, field_b = 10, }`,
		"nested objects":         `{ field_a = { nested_field = 100 } }`,
		"object multiline": `{
			field_a = 5,
			field_b = 10,
		}`,

		"unary not": `!true`,
		"unary neg": `-5`,

		"math":         `1 + 2 - 3 * 4 / 5 % 6`,
		"compare ops":  `1 == 2 != 3 < 4 > 5 <= 6 >= 7`,
		"logical ops":  `true || false && true`,
		"pow operator": "1 ^ 2 ^ 3",

		"field access":   `a.b.c.d`,
		"element access": `a[0][1][2]`,

		"call no args":             `a()`,
		"call one arg":             `a(1)`,
		"call multiple args":       `a(1,2,3)`,
		"call with trailing comma": `a(1,2,3,)`,
		"call multiline": `a(
			1,
			2,
			3,
		)`,

		"parens": `(1 + 5) * 100`,

		"mixed expression": `(a.b.c)(1, 3 * some_list[magic_index * 2]).resulting_field`,
	}

	for name, input := range tt {
		t.Run(name, func(t *testing.T) {
			p := newParser(name, []byte(input))

			res := p.ParseExpression()
			require.NotNil(t, res)
			require.Len(t, p.diags, 0)
		})
	}
}
