package scanner

import (
	"path/filepath"
	"testing"

	"github.com/grafana/river/token"
	"github.com/stretchr/testify/assert"
)

type tokenExample struct {
	tok token.Token
	lit string
}

var tokens = []tokenExample{
	// Special tokens
	{token.COMMENT, "/* a comment */"},
	{token.COMMENT, "// a comment \n"},
	{token.COMMENT, "/*\r*/"},
	{token.COMMENT, "/**\r/*/"}, // golang/go#11151
	{token.COMMENT, "/**\r\r/*/"},
	{token.COMMENT, "//\r\n"},

	// Identifiers and basic type literals
	{token.IDENT, "foobar"},
	{token.IDENT, "a۰۱۸"},
	{token.IDENT, "foo६४"},
	{token.IDENT, "bar９８７６"},
	{token.IDENT, "ŝ"},    // golang/go#4000
	{token.IDENT, "ŝfoo"}, // golang/go#4000
	{token.NUMBER, "0"},
	{token.NUMBER, "1"},
	{token.NUMBER, "123456789012345678890"},
	{token.NUMBER, "01234567"},
	{token.FLOAT, "0."},
	{token.FLOAT, ".0"},
	{token.FLOAT, "3.14159265"},
	{token.FLOAT, "1e0"},
	{token.FLOAT, "1e+100"},
	{token.FLOAT, "1e-100"},
	{token.FLOAT, "2.71828e-1000"},
	{token.STRING, `"Hello, world!"`},
	{token.STRING, "`Hello, world!\\\\`"},

	// Operators and delimiters
	{token.ADD, "+"},
	{token.SUB, "-"},
	{token.MUL, "*"},
	{token.DIV, "/"},
	{token.MOD, "%"},
	{token.POW, "^"},

	{token.AND, "&&"},
	{token.OR, "||"},

	{token.EQ, "=="},
	{token.LT, "<"},
	{token.GT, ">"},
	{token.ASSIGN, "="},
	{token.NOT, "!"},

	{token.NEQ, "!="},
	{token.LTE, "<="},
	{token.GTE, ">="},

	{token.LPAREN, "("},
	{token.LBRACK, "["},
	{token.LCURLY, "{"},
	{token.COMMA, ","},
	{token.DOT, "."},

	{token.RPAREN, ")"},
	{token.RBRACK, "]"},
	{token.RCURLY, "}"},

	// Keywords
	{token.NULL, "null"},
	{token.BOOL, "true"},
	{token.BOOL, "false"},
}

const whitespace = "  \t  \n\n\n" // Various whitespace to separate tokens

var source = func() []byte {
	var src []byte
	for _, t := range tokens {
		src = append(src, t.lit...)
		src = append(src, whitespace...)
	}
	return src
}()

// FuzzScanner ensures that the scanner will always be able to reach EOF
// regardless of input.
func FuzzScanner(f *testing.F) {
	// Add each token into the corpus
	for _, t := range tokens {
		f.Add([]byte(t.lit))
	}
	// Then add the entire source
	f.Add(source)

	f.Fuzz(func(t *testing.T, input []byte) {
		f := token.NewFile(t.Name())

		s := New(f, input, nil, IncludeComments)

		for {
			_, tok, _ := s.Scan()
			if tok == token.EOF {
				break
			}
		}
	})
}

func TestScanner_Scan(t *testing.T) {
	whitespaceLinecount := newlineCount(whitespace)

	var eh ErrorHandler = func(_ token.Pos, msg string) {
		t.Errorf("ErrorHandler called (msg = %s)", msg)
	}

	f := token.NewFile(t.Name())
	s := New(f, source, eh, IncludeComments|dontInsertTerms)

	// Configure expected position
	expectPos := token.Position{
		Filename: t.Name(),
		Offset:   0,
		Line:     1,
		Column:   1,
	}

	index := 0
	for {
		pos, tok, lit := s.Scan()

		// Check position
		checkPos(t, lit, tok, pos, expectPos)

		// Check token
		e := tokenExample{token.EOF, ""}
		if index < len(tokens) {
			e = tokens[index]
			index++
		}
		assert.Equal(t, e.tok, tok)

		// Check literal
		expectLit := ""
		switch e.tok {
		case token.COMMENT:
			// no CRs in comments
			expectLit = string(stripCR([]byte(e.lit), e.lit[1] == '*'))
			if expectLit[1] == '/' {
				// Line comment literals doesn't contain newline
				expectLit = expectLit[0 : len(expectLit)-1]
			}
		case token.IDENT:
			expectLit = e.lit
		case token.NUMBER, token.FLOAT, token.STRING, token.NULL, token.BOOL:
			expectLit = e.lit
		}
		assert.Equal(t, expectLit, lit)

		if tok == token.EOF {
			break
		}

		// Update position
		expectPos.Offset += len(e.lit) + len(whitespace)
		expectPos.Line += newlineCount(e.lit) + whitespaceLinecount
	}

	if s.NumErrors() != 0 {
		assert.Zero(t, s.NumErrors(), "expected number of scanning errors to be 0")
	}
}

func newlineCount(s string) int {
	var n int
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	return n
}

func checkPos(t *testing.T, lit string, tok token.Token, p token.Pos, expected token.Position) {
	t.Helper()

	pos := p.Position()

	// Check cleaned filenames so that we don't have to worry about different
	// os.PathSeparator values.
	if pos.Filename != expected.Filename && filepath.Clean(pos.Filename) != filepath.Clean(expected.Filename) {
		assert.Equal(t, expected.Filename, pos.Filename, "Bad filename for %s (%q)", tok, lit)
	}

	assert.Equal(t, expected.Offset, pos.Offset, "Bad offset for %s (%q)", tok, lit)
	assert.Equal(t, expected.Line, pos.Line, "Bad line for %s (%q)", tok, lit)
	assert.Equal(t, expected.Column, pos.Column, "Bad column for %s (%q)", tok, lit)
}

var errorTests = []struct {
	input string
	tok   token.Token
	pos   int
	lit   string
	err   string
}{
	{"\a", token.ILLEGAL, 0, "", "illegal character U+0007"},
	{`…`, token.ILLEGAL, 0, "", "illegal character U+2026 '…'"},
	{"..", token.DOT, 0, "", ""}, // two periods, not invalid token (golang/go#28112)
	{`'illegal string'`, token.ILLEGAL, 0, "", "illegal single-quoted string; use double quotes"},
	{`""`, token.STRING, 0, `""`, ""},
	{`"abc`, token.STRING, 0, `"abc`, "string literal not terminated"},
	{"\"abc\n", token.STRING, 0, `"abc`, "string literal not terminated"},
	{"\"abc\n   ", token.STRING, 0, `"abc`, "string literal not terminated"},
	{"\"abc\x00def\"", token.STRING, 4, "\"abc\x00def\"", "illegal character NUL"},
	{"\"abc\x80def\"", token.STRING, 4, "\"abc\x80def\"", "illegal UTF-8 encoding"},
	{"\ufeff\ufeff", token.ILLEGAL, 3, "\ufeff\ufeff", "illegal byte order mark"},                        // only first BOM is ignored
	{"//\ufeff", token.COMMENT, 2, "//\ufeff", "illegal byte order mark"},                                // only first BOM is ignored
	{`"` + "abc\ufeffdef" + `"`, token.STRING, 4, `"` + "abc\ufeffdef" + `"`, "illegal byte order mark"}, // only first BOM is ignored
	{"abc\x00def", token.IDENT, 3, "abc", "illegal character NUL"},
	{"abc\x00", token.IDENT, 3, "abc", "illegal character NUL"},
	{"10E", token.FLOAT, 0, "10E", "exponent has no digits"},
}

func TestScanner_Scan_Errors(t *testing.T) {
	for _, e := range errorTests {
		checkError(t, e.input, e.tok, e.pos, e.lit, e.err)
	}
}

func checkError(t *testing.T, src string, tok token.Token, pos int, lit, err string) {
	t.Helper()

	var (
		actualErrors int
		latestError  string
		latestPos    token.Pos
	)

	eh := func(pos token.Pos, msg string) {
		actualErrors++
		latestError = msg
		latestPos = pos
	}

	f := token.NewFile(t.Name())
	s := New(f, []byte(src), eh, IncludeComments|dontInsertTerms)

	_, actualTok, actualLit := s.Scan()

	assert.Equal(t, tok, actualTok)
	if actualTok != token.ILLEGAL {
		assert.Equal(t, lit, actualLit)
	}

	expectErrors := 0
	if err != "" {
		expectErrors = 1
	}

	assert.Equal(t, expectErrors, actualErrors, "Unexpected error count in src %q", src)
	assert.Equal(t, err, latestError, "Unexpected error message in src %q", src)
	assert.Equal(t, pos, latestPos.Offset(), "Unexpected offset in src %q", src)
}
