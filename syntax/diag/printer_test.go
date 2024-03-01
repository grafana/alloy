package diag_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/grafana/river/diag"
	"github.com/grafana/river/token"
	"github.com/stretchr/testify/require"
)

func TestFprint(t *testing.T) {
	// In all tests below, the filename is "testfile" and the severity is an
	// error.

	tt := []struct {
		name       string
		input      string
		start, end token.Position
		diag       diag.Diagnostic
		expect     string
	}{
		{
			name:  "highlight on same line",
			start: token.Position{Line: 2, Column: 2},
			end:   token.Position{Line: 2, Column: 5},
			input: `test.block "label" {
	attr       = 1
	other_attr = 2
}`,
			expect: `Error: testfile:2:2: synthetic error

1 | test.block "label" {
2 |     attr       = 1
  |     ^^^^
3 |     other_attr = 2
`,
		},

		{
			name:  "end positions should be optional",
			start: token.Position{Line: 1, Column: 4},
			input: `foo,bar`,
			expect: `Error: testfile:1:4: synthetic error

1 | foo,bar
  |    ^
`,
		},

		{
			name:  "padding should be inserted to fit line numbers of different lengths",
			start: token.Position{Line: 9, Column: 1},
			end:   token.Position{Line: 9, Column: 6},
			input: `LINE_1
LINE_2
LINE_3
LINE_4
LINE_5
LINE_6
LINE_7
LINE_8
LINE_9
LINE_10
LINE_11`,
			expect: `Error: testfile:9:1: synthetic error

 8 | LINE_8
 9 | LINE_9
   | ^^^^^^
10 | LINE_10
`,
		},

		{
			name:  "errors which cross multiple lines can be printed from start of line",
			start: token.Position{Line: 2, Column: 1},
			end:   token.Position{Line: 6, Column: 7},
			input: `FILE_BEGIN
START
TEXT
	TEXT
		TEXT
			DONE after
FILE_END`,
			expect: `Error: testfile:2:1: synthetic error

1 |   FILE_BEGIN
2 |   START
  |  _^^^^^
3 | | TEXT
4 | |     TEXT
5 | |         TEXT
6 | |             DONE after
  | |_____________^^^^
7 |   FILE_END
`,
		},

		{
			name:  "errors which cross multiple lines can be printed from middle of line",
			start: token.Position{Line: 2, Column: 8},
			end:   token.Position{Line: 6, Column: 7},
			input: `FILE_BEGIN
before START
TEXT
	TEXT
		TEXT
			DONE after
FILE_END`,
			expect: `Error: testfile:2:8: synthetic error

1 |   FILE_BEGIN
2 |   before START
  |  ________^^^^^
3 | | TEXT
4 | |     TEXT
5 | |         TEXT
6 | |             DONE after
  | |_____________^^^^
7 |   FILE_END
`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string][]byte{
				"testfile": []byte(tc.input),
			}

			tc.start.Filename = "testfile"
			tc.end.Filename = "testfile"

			diags := diag.Diagnostics{{
				Severity: diag.SeverityLevelError,
				StartPos: tc.start,
				EndPos:   tc.end,
				Message:  "synthetic error",
			}}

			var buf bytes.Buffer
			_ = diag.Fprint(&buf, files, diags)
			requireEqualStrings(t, tc.expect, buf.String())
		})
	}
}

func TestFprint_MultipleDiagnostics(t *testing.T) {
	fileA := `old_field = 15
3 & 4`
	fileB := `old_field = 22`

	files := map[string][]byte{
		"file_a": []byte(fileA),
		"file_b": []byte(fileB),
	}

	diags := diag.Diagnostics{
		{
			Severity: diag.SeverityLevelWarn,
			StartPos: token.Position{Filename: "file_a", Line: 1, Column: 1},
			EndPos:   token.Position{Filename: "file_a", Line: 1, Column: 9},
			Message:  "old_field is deprecated",
		},
		{
			Severity: diag.SeverityLevelError,
			StartPos: token.Position{Filename: "file_a", Line: 2, Column: 3},
			Message:  "unrecognized operator &",
		},
		{
			Severity: diag.SeverityLevelWarn,
			StartPos: token.Position{Filename: "file_b", Line: 1, Column: 1},
			EndPos:   token.Position{Filename: "file_b", Line: 1, Column: 9},
			Message:  "old_field is deprecated",
		},
	}

	expect := `Warning: file_a:1:1: old_field is deprecated

1 | old_field = 15
  | ^^^^^^^^^
2 | 3 & 4

Error: file_a:2:3: unrecognized operator &

1 | old_field = 15
2 | 3 & 4
  |   ^

Warning: file_b:1:1: old_field is deprecated

1 | old_field = 22
  | ^^^^^^^^^
`

	var buf bytes.Buffer
	_ = diag.Fprint(&buf, files, diags)
	requireEqualStrings(t, expect, buf.String())
}

// requireEqualStrings is like require.Equal with two strings but it
// pretty-prints multiline strings to make it easier to compare.
func requireEqualStrings(t *testing.T, expected, actual string) {
	if expected == actual {
		return
	}

	msg := fmt.Sprintf(
		"Not equal:\n"+
			"raw expected: %#v\n"+
			"raw actual  : %#v\n"+
			"\n"+
			"expected:\n%s\n"+
			"actual:\n%s\n",
		expected, actual,
		expected, actual,
	)

	require.Fail(t, msg)
}
