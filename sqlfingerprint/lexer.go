package sqlfingerprint

import "strings"

// tokenKind separates SQL syntax, identifiers, and erased values in the hash.
type tokenKind byte

const (
	word tokenKind = iota
	identifier
	value
	number
	symbol
	ellipsis
	listComment
	valueList
	rowSingle
	rowMultiple
	rowSingleList
	rowMultipleList
)

type token struct {
	kind tokenKind
	text string
}

// lexer owns the cursor for one batch. Only semicolons outside quoted regions
// and comments reach statement; SQL strings are never split with a regex.
type lexer struct {
	input   string
	pos     int
	dialect Dialect
	options Options
}

// statement reads one nonempty statement. Closed unsupported constructs fail
// only this statement; an open quote or comment makes the remaining batch unsafe.
func (l *lexer) statement() ([]token, Reason, bool) {
	var tokens []token
	var reason Reason
	for l.pos < len(l.input) {
		t, problem, fatal := l.next()
		if problem != "" {
			reason = problem
		}
		if fatal {
			return tokens, reason, true
		}
		if t == (token{symbol, ";"}) {
			if len(tokens) > 0 || reason != "" {
				return tokens, reason, false
			}
			continue
		}
		if t.text != "" {
			tokens = append(tokens, t)
		}
		if len(tokens) > 65536 {
			return nil, Limit, true
		}
	}
	return tokens, reason, true
}

// next dispatches lexical constructs before consuming individual punctuation.
func (l *lexer) next() (token, Reason, bool) {
	c := l.input[l.pos]
	if isSpace(c) {
		l.pos++
		return token{}, "", false
	}
	if l.lineComment() {
		return token{}, "", false
	}
	if strings.HasPrefix(l.input[l.pos:], "/*") {
		return l.blockComment()
	}
	if l.dialect == PostgreSQL && c == '$' {
		return l.dollar()
	}
	if c == '\'' || c == '"' || c == '`' || (c == '[' && l.dialect == SQLServer) {
		t, reason, fatal := l.quoted(c, false)
		if c == '`' && l.dialect != MySQL && reason == "" {
			reason = Unsupported
		}
		return t, reason, fatal
	}
	if isDigit(c) || (c == '.' && l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1])) {
		return l.numeric()
	}
	if c == '@' && l.dialect != PostgreSQL {
		return l.variable()
	}
	if isWordStart(c) || (c == '#' && l.dialect == SQLServer) {
		return l.name()
	}
	return l.punctuation()
}

// lineComment implements MySQL's whitespace requirement after --, so a--1
// remains subtraction, as well as its # comment syntax.
func (l *lexer) lineComment() bool {
	rest := l.input[l.pos:]
	starts := strings.HasPrefix(rest, "--")
	if l.dialect == MySQL {
		starts = (starts && (len(rest) == 2 || isSpace(rest[2]))) || rest[0] == '#'
	}
	if !starts {
		return false
	}
	for l.pos < len(l.input) && l.input[l.pos] != '\n' && l.input[l.pos] != '\r' {
		l.pos++
	}
	return true
}

// blockComment tracks nesting where the engine permits it. MySQL executable
// comments and optimizer hints fail closed instead of silently losing meaning.
func (l *lexer) blockComment() (token, Reason, bool) {
	start := l.pos + 2
	l.pos = start
	depth := 1
	for l.pos < len(l.input)-1 {
		rest := l.input[l.pos:]
		if strings.HasPrefix(rest, "/*") && l.dialect != MySQL {
			depth++
			l.pos += 2
		} else if strings.HasPrefix(rest, "*/") {
			depth--
			end := l.pos
			l.pos += 2
			if depth == 0 {
				return commentToken(l.input[start:end])
			}
		} else {
			l.pos++
		}
	}
	return token{}, InvalidSQL, true
}

// commentToken recognizes only an exact native list marker, not arbitrary
// comments containing an ellipsis. Diagnostics never include comment text.
func commentToken(body string) (token, Reason, bool) {
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "!") || strings.HasPrefix(trimmed, "+") {
		return token{}, Unsupported, false
	}
	if strings.Join(strings.Fields(body), "") == ",..." {
		return token{listComment, "..."}, "", false
	}
	return token{}, "", false
}

// quoted handles doubled delimiters and configured string escapes. It decodes
// identifiers without changing their case and discards literal contents.
func (l *lexer) quoted(open byte, escape bool) (token, Reason, bool) {
	close := open
	if open == '[' {
		close = ']'
	}
	kind := l.quoteKind(open)
	escape = escape || (kind == value && l.backslashEscapes())
	l.pos++
	var name strings.Builder
	for l.pos < len(l.input) {
		c := l.input[l.pos]
		l.pos++
		if c == close {
			if l.pos < len(l.input) && l.input[l.pos] == close {
				l.pos++
			} else {
				if kind == value {
					return token{value, "?"}, "", false
				}
				return token{identifier, name.String()}, "", false
			}
		} else if c == '\\' && escape && l.pos < len(l.input) {
			c = l.input[l.pos]
			l.pos++
		}
		if kind == identifier {
			name.WriteByte(c)
		}
	}
	return token{}, InvalidSQL, true
}

// quoteKind resolves double quotes using the configured engine/session mode.
func (l *lexer) quoteKind(open byte) tokenKind {
	if open == '\'' || (open == '"' && ((l.dialect == MySQL && !l.options.MySQLANSIQuotes) ||
		(l.dialect == SQLServer && l.options.SQLServerQuotedIdentifierOff))) {

		return value
	}
	return identifier
}

// backslashEscapes reports whether ordinary strings interpret backslashes.
func (l *lexer) backslashEscapes() bool {
	return (l.dialect == MySQL && !l.options.MySQLNoBackslashEscapes) ||
		(l.dialect == PostgreSQL && l.options.PostgreSQLBackslashEscapes)
}

// dollar distinguishes PostgreSQL positional parameters from dollar strings.
func (l *lexer) dollar() (token, Reason, bool) {
	start := l.pos
	l.pos++
	if l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
		return token{value, "?"}, "", false
	}
	for l.pos < len(l.input) && (isWordStart(l.input[l.pos]) || isDigit(l.input[l.pos])) {
		l.pos++
	}
	if l.pos >= len(l.input) || l.input[l.pos] != '$' {
		return token{}, InvalidSQL, true
	}
	l.pos++
	tag := l.input[start:l.pos]
	end := strings.Index(l.input[l.pos:], tag)
	if end < 0 {
		return token{}, InvalidSQL, true
	}
	l.pos += end + len(tag)
	return token{value, "?"}, "", false
}

// name preserves identifier spelling and consumes recognized string prefixes
// with their literal. Unicode escape identifiers await explicit decoding support.
func (l *lexer) name() (token, Reason, bool) {
	start := l.pos
	l.pos++
	for l.pos < len(l.input) && (isWordPart(l.input[l.pos]) || (l.input[l.pos] == '#' && l.dialect == SQLServer)) {
		l.pos++
	}
	name := l.input[start:l.pos]
	upper := strings.ToUpper(name)
	if l.pos < len(l.input) && l.input[l.pos] == '\'' && isLiteralPrefix(upper, l.dialect) {
		return l.quoted('\'', l.dialect == PostgreSQL && upper == "E")
	}
	if upper == "U" && strings.HasPrefix(l.input[l.pos:], "&") {
		return token{word, name}, Unsupported, false
	}
	return token{word, name}, "", false
}

// isLiteralPrefix recognizes the dialect's prefixes attached to single quotes.
func isLiteralPrefix(s string, dialect Dialect) bool {
	switch dialect {
	case PostgreSQL:
		return s == "E" || s == "B" || s == "X" || s == "N"
	case MySQL:
		return s == "B" || s == "X" || s == "N"
	default:
		return s == "N"
	}
}

// numeric reads decimal, exponent, hexadecimal, and binary literals. Signs
// remain separate so normalization can distinguish subtraction from -1.
func (l *lexer) numeric() (token, Reason, bool) {
	prefix := l.input[l.pos:min(l.pos+2, len(l.input))]
	valid := true
	if strings.EqualFold(prefix, "0x") || strings.EqualFold(prefix, "0b") {
		l.pos += 2
		start := l.pos
		for l.pos < len(l.input) && strings.ContainsRune("0123456789abcdefABCDEF_", rune(l.input[l.pos])) {
			if strings.EqualFold(prefix, "0b") && !strings.ContainsRune("01_", rune(l.input[l.pos])) {
				valid = false
			}
			l.pos++
		}
		valid = valid && l.pos > start
	} else {
		valid = l.decimal()
	}
	if !valid || (l.pos < len(l.input) && isWordStart(l.input[l.pos])) {
		return token{}, InvalidSQL, false
	}
	return token{number, "?"}, "", false
}

// decimal reads an unsigned decimal and requires digits in any exponent.
func (l *lexer) decimal() bool {
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.pos++
	}
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		l.pos++
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	if l.pos < len(l.input) && (l.input[l.pos] == 'e' || l.input[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
			l.pos++
		}
		start := l.pos
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
		return l.pos > start
	}
	return true
}

// variable erases SQL Server bind names but retains server variables and MySQL
// user variables, whose names survive in native statement statistics.
func (l *lexer) variable() (token, Reason, bool) {
	start := l.pos
	l.pos++
	for l.pos < len(l.input) && (isWordPart(l.input[l.pos]) || l.input[l.pos] == '@') {
		l.pos++
	}
	name := l.input[start:l.pos]
	if len(name) == 1 {
		return token{}, InvalidSQL, false
	}
	if l.dialect == SQLServer && !strings.HasPrefix(name, "@@") {
		return token{parameter, "?"}, "", false
	}
	return token{identifier, name}, "", false
}

// punctuation prefers longer operators. PostgreSQL's ? JSON operator survives;
// dialects with ? bind markers instead normalize it as an erased value.
func (l *lexer) punctuation() (token, Reason, bool) {
	rest := l.input[l.pos:]
	if strings.HasPrefix(rest, "...") {
		l.pos += 3
		return token{ellipsis, "..."}, "", false
	}
	for _, op := range []string{"->>", "#>>", "<=>", "!~*", "::", ">=", "<=", "<>", "!=", "||", "&&", "->", "#>", "@>", "<@", "?|", "?&", "#-", "!~", "~*", ":=", "<<", ">>", "!<", "!>"} {
		if strings.HasPrefix(rest, op) {
			l.pos += len(op)
			return token{symbol, op}, "", false
		}
	}
	l.pos++
	if rest[0] == '?' && l.dialect != PostgreSQL {
		return token{value, "?"}, "", false
	}
	if strings.ContainsRune("();,.[]+-*/%=<>!~^|&?#", rune(rest[0])) {
		return token{symbol, rest[:1]}, "", false
	}
	return token{}, Unsupported, false
}

// isSpace recognizes SQL's ASCII whitespace.
func isSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }

// isDigit recognizes ASCII numeric and parameter-index digits.
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isWordStart preserves UTF-8 identifier bytes after whole-input validation.
func isWordStart(c byte) bool {
	return c == '_' || c >= 0x80 || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isWordPart also accepts digits and embedded dollar signs in identifiers.
func isWordPart(c byte) bool { return isWordStart(c) || isDigit(c) || c == '$' }
