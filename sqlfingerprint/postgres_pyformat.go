package sqlfingerprint

// postgresNamedParameter reads a Python %(name)s placeholder as a value token.
// The caller requires an operand position so a native modulo expression followed
// by an alias, such as rating%(scale)s, keeps its operators and identifiers.
func (l *lexer) postgresNamedParameter() (token, Reason, bool) {
	l.pos += 2 // The caller has already recognized the opening %( sequence.
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != ')' {
		// These characters can't occur in Psycopg mapping parameter names.
		if l.input[l.pos] == '%' || l.input[l.pos] == '(' {
			return token{}, InvalidSQL, false
		}
		l.pos++
	}
	if l.pos == len(l.input) {
		return token{}, InvalidSQL, true
	}
	empty := l.pos == start
	l.pos++ // Consume the closing parenthesis, leaving any cast for the lexer.
	if empty || l.pos == len(l.input) || l.input[l.pos] != 's' {
		return token{}, InvalidSQL, false
	}
	l.pos++
	return token{value, "?"}, "", false
}
