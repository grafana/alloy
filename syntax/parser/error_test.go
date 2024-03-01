package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/grafana/river/diag"
	"github.com/grafana/river/scanner"
	"github.com/grafana/river/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file implements a parser test harness. The files in the testdata
// directory are parsed and the errors reported are compared against the error
// messages expected in the test files.
//
// Expected errors are indicated in the test files by putting a comment of the
// form /* ERROR "rx" */ immediately following an offending token. The harness
// will verify that an error matching the regular expression rx is reported at
// that source position.

// ERROR comments must be of the form /* ERROR "rx" */ and rx is a regular
// expression that matches the expected error message. The special form
// /* ERROR HERE "rx" */ must be used for error messages that appear immediately
// after a token rather than at a token's position.
var errRx = regexp.MustCompile(`^/\* *ERROR *(HERE)? *"([^"]*)" *\*/$`)

// expectedErrors collects the regular expressions of ERROR comments found in
// files and returns them as a map of error positions to error messages.
func expectedErrors(file *token.File, src []byte) map[token.Pos]string {
	errors := make(map[token.Pos]string)

	s := scanner.New(file, src, nil, scanner.IncludeComments)

	var (
		prev token.Pos // Position of last non-comment, non-terminator token
		here token.Pos // Position following after token at prev
	)

	for {
		pos, tok, lit := s.Scan()
		switch tok {
		case token.EOF:
			return errors
		case token.COMMENT:
			s := errRx.FindStringSubmatch(lit)
			if len(s) == 3 {
				pos := prev
				if s[1] == "HERE" {
					pos = here
				}
				errors[pos] = s[2]
			}
		case token.TERMINATOR:
			if lit == "\n" {
				break
			}
			fallthrough
		default:
			prev = pos
			var l int // Token length
			if isLiteral(tok) {
				l = len(lit)
			} else {
				l = len(tok.String())
			}
			here = prev.Add(l)
		}
	}
}

func isLiteral(t token.Token) bool {
	switch t {
	case token.IDENT, token.NUMBER, token.FLOAT, token.STRING:
		return true
	}
	return false
}

// compareErrors compares the map of expected error messages with the list of
// found errors and reports mismatches.
func compareErrors(t *testing.T, file *token.File, expected map[token.Pos]string, found diag.Diagnostics) {
	t.Helper()

	for _, checkError := range found {
		pos := file.Pos(checkError.StartPos.Offset)

		if msg, found := expected[pos]; found {
			// We expect a message at pos; check if it matches
			rx, err := regexp.Compile(msg)
			if !assert.NoError(t, err) {
				continue
			}
			assert.True(t,
				rx.MatchString(checkError.Message),
				"%s: %q does not match %q",
				checkError.StartPos, checkError.Message, msg,
			)
			delete(expected, pos) // Eliminate consumed error
		} else {
			assert.Fail(t,
				"Unexpected error",
				"unexpected error: %s: %s", checkError.StartPos.String(), checkError.Message,
			)
		}
	}

	// There should be no expected errors left
	if len(expected) > 0 {
		t.Errorf("%d errors not reported:", len(expected))
		for pos, msg := range expected {
			t.Errorf("%s: %s\n", file.PositionFor(pos), msg)
		}
	}
}

func TestErrors(t *testing.T) {
	list, err := os.ReadDir("testdata")
	require.NoError(t, err)

	for _, d := range list {
		name := d.Name()
		if d.IsDir() || !strings.HasSuffix(name, ".river") {
			continue
		}

		t.Run(name, func(t *testing.T) {
			checkErrors(t, filepath.Join("testdata", name))
		})
	}
}

func checkErrors(t *testing.T, filename string) {
	t.Helper()

	src, err := os.ReadFile(filename)
	require.NoError(t, err)

	p := newParser(filename, src)
	_ = p.ParseFile()

	expected := expectedErrors(p.file, src)
	compareErrors(t, p.file, expected, p.diags)
}
