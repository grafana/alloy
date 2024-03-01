// Package scanner implements a lexical scanner for River source files.
package scanner

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/grafana/river/token"
)

// EBNF for the scanner:
//
//   letter           = /* any unicode letter class character */ | "_"
//   number           = /* any unicode number class character */
//   digit            = /* ASCII characters 0 through 9 */
//   digits           = digit { digit }
//   string_character = /* any unicode character that isn't '"' */
//
//   COMMENT       = line_comment | block_comment
//   line_comment  = "//" { character }
//   block_comment = "/*" { character | newline } "*/"
//
//   IDENT   = letter { letter | number }
//   NULL    = "null"
//   BOOL    = "true" | "false"
//   NUMBER  = digits
//   FLOAT   = ( digits | "." digits ) [ "e" [ "+" | "-" ] digits ]
//   STRING  = '"' { string_character | escape_sequence } '"'
//   OR      = "||"
//   AND     = "&&"
//   NOT     = "!"
//   NEQ     = "!="
//   ASSIGN  = "="
//   EQ      = "=="
//   LT      = "<"
//   LTE     = "<="
//   GT      = ">"
//   GTE     = ">="
//   ADD     = "+"
//   SUB     = "-"
//   MUL     = "*"
//   DIV     = "/"
//   MOD     = "%"
//   POW     = "^"
//   LCURLY  = "{"
//   RCURLY  = "}"
//   LPAREN  = "("
//   RPAREN  = ")"
//   LBRACK  = "["
//   RBRACK  = "]"
//   COMMA   = ","
//   DOT     = "."
//
// The EBNF for escape_sequence is currently undocumented; see scanEscape for
// details. The escape sequences supported by River are the same as the escape
// sequences supported by Go, except that it is always valid to use \' in
// strings (which in Go, is only valid to use in character literals).

// ErrorHandler is invoked whenever there is an error.
type ErrorHandler func(pos token.Pos, msg string)

// Mode is a set of bitwise flags which control scanner behavior.
type Mode uint

const (
	// IncludeComments will cause comments to be returned as comment tokens.
	// Otherwise, comments are ignored.
	IncludeComments Mode = 1 << iota

	// Avoids automatic insertion of terminators (for testing only).
	dontInsertTerms
)

const (
	bom = 0xFEFF // byte order mark, permitted as very first character
	eof = -1     // end of file
)

// Scanner holds the internal state for the tokenizer while processing configs.
type Scanner struct {
	file  *token.File  // Config file handle for tracking line offsets
	input []byte       // Input config
	err   ErrorHandler // Error reporting (may be nil)
	mode  Mode

	// scanning state variables:

	ch         rune // Current character
	offset     int  // Byte offset of ch
	readOffset int  // Byte offset of first character *after* ch
	insertTerm bool // Insert a newline before the next newline
	numErrors  int  // Number of errors encountered during scanning
}

// New creates a new scanner to tokenize the provided input config. The scanner
// uses the provided file for adding line information for each token. The mode
// parameter customizes scanner behavior.
//
// Calls to Scan will invoke the error handler eh when a lexical error is found
// if eh is not nil.
func New(file *token.File, input []byte, eh ErrorHandler, mode Mode) *Scanner {
	s := &Scanner{
		file:  file,
		input: input,
		err:   eh,
		mode:  mode,
	}

	// Preload first character.
	s.next()
	if s.ch == bom {
		s.next() // Ignore BOM if it's the first character.
	}
	return s
}

// peek gets the next byte after the current character without advancing the
// scanner. Returns 0 if the scanner is at EOF.
func (s *Scanner) peek() byte {
	if s.readOffset < len(s.input) {
		return s.input[s.readOffset]
	}
	return 0
}

// next advances the scanner and reads the next Unicode character into s.ch.
// s.ch == eof indicates end of file.
func (s *Scanner) next() {
	if s.readOffset >= len(s.input) {
		s.offset = len(s.input)
		if s.ch == '\n' {
			// Make sure we track final newlines at the end of the file
			s.file.AddLine(s.offset)
		}
		s.ch = eof
		return
	}

	s.offset = s.readOffset
	if s.ch == '\n' {
		s.file.AddLine(s.offset)
	}

	r, width := rune(s.input[s.readOffset]), 1
	switch {
	case r == 0:
		s.onError(s.offset, "illegal character NUL")
	case r >= utf8.RuneSelf:
		r, width = utf8.DecodeRune(s.input[s.readOffset:])
		if r == utf8.RuneError && width == 1 {
			s.onError(s.offset, "illegal UTF-8 encoding")
		} else if r == bom && s.offset > 0 {
			s.onError(s.offset, "illegal byte order mark")
		}
	}
	s.readOffset += width
	s.ch = r
}

func (s *Scanner) onError(offset int, msg string) {
	if s.err != nil {
		s.err(s.file.Pos(offset), msg)
	}
	s.numErrors++
}

// NumErrors returns the current number of errors encountered during scanning.
// This is useful as a fallback to detect errors when no ErrorHandler was
// provided to the scanner.
func (s *Scanner) NumErrors() int { return s.numErrors }

// Scan scans the next token and returns the token's position, the token
// itself, and the token's literal string (when applicable). The end of the
// input is indicated by token.EOF.
//
// If the returned token is a literal (such as token.STRING), then lit contains
// the corresponding literal text (including surrounding quotes).
//
// If the returned token is a keyword, lit is the keyword text that was
// scanned.
//
// If the returned token is token.TERMINATOR, lit will contain "\n".
//
// If the returned token is token.ILLEGAL, lit contains the offending
// character.
//
// In all other cases, lit will be an empty string.
//
// For more tolerant parsing, Scan returns a valid token character whenever
// possible when a syntax error was encountered. Callers must check NumErrors
// or the number of times the provided ErrorHandler was invoked to ensure there
// were no errors found during scanning.
//
// Scan will inject line information to the file provided by NewScanner.
// Returned token positions are relative to that file.
func (s *Scanner) Scan() (pos token.Pos, tok token.Token, lit string) {
scanAgain:
	s.skipWhitespace()

	// Start of current token.
	pos = s.file.Pos(s.offset)

	var insertTerm bool

	// Determine token value
	switch ch := s.ch; {
	case isLetter(ch):
		lit = s.scanIdentifier()
		if len(lit) > 1 { // Keywords are always > 1 char
			tok = token.Lookup(lit)
			switch tok {
			case token.IDENT, token.NULL, token.BOOL:
				insertTerm = true
			}
		} else {
			insertTerm = true
			tok = token.IDENT
		}

	case isDecimal(ch) || (ch == '.' && isDecimal(rune(s.peek()))):
		insertTerm = true
		tok, lit = s.scanNumber()

	default:
		s.next() // Make progress

		// ch is now the first character in a sequence and s.ch is the second
		// character.

		switch ch {
		case eof:
			if s.insertTerm {
				s.insertTerm = false // Consumed EOF
				return pos, token.TERMINATOR, "\n"
			}
			tok = token.EOF

		case '\n':
			// This case is only reachable when s.insertTerm is true, since otherwise
			// skipWhitespace consumes all other newlines.
			s.insertTerm = false // Consumed newline
			return pos, token.TERMINATOR, "\n"

		case '\'':
			s.onError(pos.Offset(), "illegal single-quoted string; use double quotes")
			insertTerm = true
			tok = token.ILLEGAL
			lit = s.scanString('\'', true, false)

		case '"':
			insertTerm = true
			tok = token.STRING
			lit = s.scanString('"', true, false)

		case '`':
			insertTerm = true
			tok = token.STRING
			lit = s.scanString('`', false, true)

		case '|':
			if s.ch != '|' {
				s.onError(s.offset, "missing second | in ||")
			} else {
				s.next() // consume second '|'
			}
			tok = token.OR
		case '&':
			if s.ch != '&' {
				s.onError(s.offset, "missing second & in &&")
			} else {
				s.next() // consume second '&'
			}
			tok = token.AND

		case '!': // !, !=
			tok = s.switch2(token.NOT, token.NEQ, '=')
		case '=': // =, ==
			tok = s.switch2(token.ASSIGN, token.EQ, '=')
		case '<': // <, <=
			tok = s.switch2(token.LT, token.LTE, '=')
		case '>': // >, >=
			tok = s.switch2(token.GT, token.GTE, '=')
		case '+':
			tok = token.ADD
		case '-':
			tok = token.SUB
		case '*':
			tok = token.MUL
		case '/':
			if s.ch == '/' || s.ch == '*' {
				// //- or /*-style comment.
				//
				// If we're expected to inject a terminator, we can only do so if our
				// comment goes to the end of the line. Otherwise, the terminator will
				// have to be injected after the comment token.
				if s.insertTerm && s.findLineEnd() {
					// Reset position to the beginning of the comment.
					s.ch = '/'
					s.offset = pos.Offset()
					s.readOffset = s.offset + 1
					s.insertTerm = false // Consumed newline
					return pos, token.TERMINATOR, "\n"
				}
				comment := s.scanComment()
				if s.mode&IncludeComments == 0 {
					// Skip over comment
					s.insertTerm = false // Consumed newline
					goto scanAgain
				}
				tok = token.COMMENT
				lit = comment
			} else {
				tok = token.DIV
			}

		case '%':
			tok = token.MOD
		case '^':
			tok = token.POW
		case '{':
			tok = token.LCURLY
		case '}':
			insertTerm = true
			tok = token.RCURLY
		case '(':
			tok = token.LPAREN
		case ')':
			insertTerm = true
			tok = token.RPAREN
		case '[':
			tok = token.LBRACK
		case ']':
			insertTerm = true
			tok = token.RBRACK
		case ',':
			tok = token.COMMA
		case '.':
			// NOTE: Fractions starting with '.' are handled by outer switch
			tok = token.DOT

		default:
			// s.next() reports invalid BOMs so we don't need to repeat the error.
			if ch != bom {
				s.onError(pos.Offset(), fmt.Sprintf("illegal character %#U", ch))
			}
			insertTerm = s.insertTerm // Preserve previous s.insertTerm state
			tok = token.ILLEGAL
			lit = string(ch)
		}
	}

	if s.mode&dontInsertTerms == 0 {
		s.insertTerm = insertTerm
	}
	return
}

func (s *Scanner) skipWhitespace() {
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\r' || (s.ch == '\n' && !s.insertTerm) {
		s.next()
	}
}

func isLetter(ch rune) bool {
	// We check for ASCII first as an optimization, and leave checking unicode
	// (the slowest) to the very end.
	return (lower(ch) >= 'a' && lower(ch) <= 'z') ||
		ch == '_' ||
		(ch >= utf8.RuneSelf && unicode.IsLetter(ch))
}

func lower(ch rune) rune     { return ('a' - 'A') | ch }
func isDecimal(ch rune) bool { return '0' <= ch && ch <= '9' }
func isDigit(ch rune) bool {
	return isDecimal(ch) || (ch >= utf8.RuneSelf && unicode.IsDigit(ch))
}

// scanIdentifier reads the string of valid identifier characters starting at
// s.offet. It must only be called when s.ch is a valid character which starts
// an identifier.
//
// scanIdentifier is highly optimized for identifiers are modifications must be
// made carefully.
func (s *Scanner) scanIdentifier() string {
	off := s.offset

	// Optimize for common case of ASCII identifiers.
	//
	// Ranging over s.input[s.readOffset:] avoids bounds checks and avoids
	// conversions to runes.
	//
	// We'll fall back to the slower path if we find a non-ASCII character.
	for readOffset, b := range s.input[s.readOffset:] {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || (b >= '0' && b <= '9') {
			// Common case: ASCII character; don't assign a rune.
			continue
		}
		s.readOffset += readOffset
		if b > 0 && b < utf8.RuneSelf {
			// Optimization: ASCII character that isn't a letter or number; we've
			// reached the end of the identifier sequence and can terminate. We avoid
			// the call to s.next() and the corresponding setup.
			//
			// This optimization only works because we know that s.ch (the current
			// character when scanIdentifier was called) is never '\n' since '\n'
			// cannot start an identifier.
			s.ch = rune(b)
			s.offset = s.readOffset
			s.readOffset++
			goto exit
		}

		// The preceding character is valid for an identifier because
		// scanIdentifier is only called when s.ch is a letter; calling s.next() at
		// s.readOffset will reset the scanner state.
		s.next()
		for isLetter(s.ch) || isDigit(s.ch) {
			s.next()
		}

		// No more valid characters for the identifier; terminate.
		goto exit
	}

	s.offset = len(s.input)
	s.readOffset = len(s.input)
	s.ch = eof

exit:
	return string(s.input[off:s.offset])
}

func (s *Scanner) scanNumber() (tok token.Token, lit string) {
	tok = token.NUMBER
	off := s.offset

	// Integer part of number
	if s.ch != '.' {
		s.digits()
	}

	// Fractional part of number
	if s.ch == '.' {
		tok = token.FLOAT

		s.next()
		s.digits()
	}

	// Exponent
	if lower(s.ch) == 'e' {
		tok = token.FLOAT

		s.next()
		if s.ch == '+' || s.ch == '-' {
			s.next()
		}

		if s.digits() == 0 {
			s.onError(off, "exponent has no digits")
		}
	}

	return tok, string(s.input[off:s.offset])
}

// digits scans a sequence of digits.
func (s *Scanner) digits() (count int) {
	for isDecimal(s.ch) {
		s.next()
		count++
	}
	return
}

func (s *Scanner) scanString(until rune, escape bool, multiline bool) string {
	// subtract 1 to account for the opening '"' which was already consumed by
	// the scanner forcing progress.
	off := s.offset - 1

	for {
		ch := s.ch
		if (!multiline && ch == '\n') || ch == eof {
			s.onError(off, "string literal not terminated")
			break
		}
		s.next()
		if ch == until {
			break
		}
		if escape && ch == '\\' {
			s.scanEscape()
		}
	}

	return string(s.input[off:s.offset])
}

// scanEscape parses an escape sequence. In case of a syntax error, scanEscape
// stops at the offending character without consuming it.
func (s *Scanner) scanEscape() {
	off := s.offset

	var (
		n         int
		base, max uint32
	)

	switch s.ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', '"':
		s.next()
		return
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, max = 3, 8, 255
	case 'x':
		s.next()
		n, base, max = 2, 16, 255
	case 'u':
		s.next()
		n, base, max = 4, 16, unicode.MaxRune
	case 'U':
		s.next()
		n, base, max = 8, 16, unicode.MaxRune
	default:
		msg := "unknown escape sequence"
		if s.ch == eof {
			msg = "escape sequence not terminated"
		}
		s.onError(off, msg)
		return
	}

	var x uint32
	for n > 0 {
		d := uint32(digitVal(s.ch))
		if d >= base {
			msg := fmt.Sprintf("illegal character %#U in escape sequence", s.ch)
			if s.ch == eof {
				msg = "escape sequence not terminated"
			}
			s.onError(off, msg)
			return
		}
		x = x*base + d
		s.next()
		n--
	}

	if x > max || x >= 0xD800 && x < 0xE000 {
		s.onError(off, "escape sequence is invalid Unicode code point")
	}
}

func digitVal(ch rune) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case lower(ch) >= 'a' && lower(ch) <= 'f':
		return int(lower(ch) - 'a' + 10)
	}
	return 16 // Larger than any legal digit val
}

func (s *Scanner) scanComment() string {
	// The initial character in the comment was already consumed from the scanner
	// forcing progress.
	//
	// slashComment will be true when the comment is a //- or /*-style comment.

	var (
		off   = s.offset - 1 // Offset of initial character
		numCR = 0

		blockComment = false
	)

	if s.ch == '/' { // NOTE: s.ch is second character in comment sequence
		// //-style comment.
		//
		// The final '\n' is not considered to be part of the comment.
		if s.ch == '/' {
			s.next() // Consume second '/'
		}

		for s.ch != '\n' && s.ch != eof {
			if s.ch == '\r' {
				numCR++
			}
			s.next()
		}

		goto exit
	}

	// /*-style comment.
	blockComment = true
	s.next()
	for s.ch != eof {
		ch := s.ch
		if ch == '\r' {
			numCR++
		}
		s.next()
		if ch == '*' && s.ch == '/' {
			s.next()
			goto exit
		}
	}

	s.onError(off, "block comment not terminated")

exit:
	lit := s.input[off:s.offset]

	// On Windows, a single comment line may end in "\r\n". We want to remove the
	// final \r.
	if numCR > 0 && len(lit) >= 1 && lit[len(lit)-1] == '\r' {
		lit = lit[:len(lit)-1]
		numCR--
	}

	if numCR > 0 {
		lit = stripCR(lit, blockComment)
	}

	return string(lit)
}

func stripCR(b []byte, blockComment bool) []byte {
	c := make([]byte, len(b))
	i := 0

	for j, ch := range b {
		if ch != '\r' || blockComment && i > len("/*") && c[i-1] == '*' && j+1 < len(b) && b[j+1] == '/' {
			c[i] = ch
			i++
		}
	}

	return c[:i]
}

// findLineEnd checks to see if a comment runs to the end of the line.
func (s *Scanner) findLineEnd() bool {
	// NOTE: initial '/' is already consumed by forcing the scanner to progress.

	defer func(off int) {
		// Reset scanner state to where it was upon calling findLineEnd.
		s.ch = '/'
		s.offset = off
		s.readOffset = off + 1
		s.next() // Consume initial starting '/' again
	}(s.offset - 1)

	// Read ahead until a newline, EOF, or non-comment token is found.
	// We loop to consume multiple sequences of comment tokens.
	for s.ch == '/' || s.ch == '*' {
		if s.ch == '/' {
			// //-style comments always contain newlines.
			return true
		}

		// We're looking at a /*-style comment; look for its newline.
		s.next()
		for s.ch != eof {
			ch := s.ch
			if ch == '\n' {
				return true
			}
			s.next()
			if ch == '*' && s.ch == '/' { // End of block comment
				s.next()
				break
			}
		}

		// Check to see if there's a newline after the block comment.
		s.skipWhitespace() // s.insertTerm is set
		if s.ch == eof || s.ch == '\n' {
			return true
		}
		if s.ch != '/' {
			// Non-comment token
			return false
		}
		s.next() // Consume '/' at the end of the /* style-comment
	}

	return false
}

// switch2 returns a if s.ch is next, b otherwise. The scanner will be advanced
// if b is returned.
//
// This is used for tokens which can either be a single character but also are
// the starting character for a 2-length token (i.e., = and ==).
func (s *Scanner) switch2(a, b token.Token, next rune) token.Token { //nolint:unparam
	if s.ch == next {
		s.next()
		return b
	}
	return a
}
