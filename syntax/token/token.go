// Package token defines the lexical elements of a River config and utilities
// surrounding their position.
package token

// Token is an individual River lexical token.
type Token int

// List of all lexical tokens and examples that represent them.
//
// LITERAL is used by token/builder to represent literal strings for writing
// tokens, but never used for reading (so scanner never returns a
// token.LITERAL).
const (
	ILLEGAL Token = iota // Invalid token.
	LITERAL              // Literal text.
	EOF                  // End-of-file.
	COMMENT              // // Hello, world!

	literalBeg
	IDENT  // foobar
	NUMBER // 1234
	FLOAT  // 1234.0
	STRING // "foobar"
	literalEnd

	keywordBeg
	BOOL // true
	NULL // null
	keywordEnd

	operatorBeg
	OR  // ||
	AND // &&
	NOT // !

	ASSIGN // =

	EQ  // ==
	NEQ // !=
	LT  // <
	LTE // <=
	GT  // >
	GTE // >=

	ADD // +
	SUB // -
	MUL // *
	DIV // /
	MOD // %
	POW // ^

	LCURLY // {
	RCURLY // }
	LPAREN // (
	RPAREN // )
	LBRACK // [
	RBRACK // ]
	COMMA  // ,
	DOT    // .
	operatorEnd

	TERMINATOR // \n
)

var tokenNames = [...]string{
	ILLEGAL: "ILLEGAL",
	LITERAL: "LITERAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",

	IDENT:  "IDENT",
	NUMBER: "NUMBER",
	FLOAT:  "FLOAT",
	STRING: "STRING",
	BOOL:   "BOOL",
	NULL:   "NULL",

	OR:  "||",
	AND: "&&",
	NOT: "!",

	ASSIGN: "=",
	EQ:     "==",
	NEQ:    "!=",
	LT:     "<",
	LTE:    "<=",
	GT:     ">",
	GTE:    ">=",

	ADD: "+",
	SUB: "-",
	MUL: "*",
	DIV: "/",
	MOD: "%",
	POW: "^",

	LCURLY: "{",
	RCURLY: "}",
	LPAREN: "(",
	RPAREN: ")",
	LBRACK: "[",
	RBRACK: "]",
	COMMA:  ",",
	DOT:    ".",

	TERMINATOR: "TERMINATOR",
}

// Lookup maps a string to its keyword token or IDENT if it's not a keyword.
func Lookup(ident string) Token {
	switch ident {
	case "true", "false":
		return BOOL
	case "null":
		return NULL
	default:
		return IDENT
	}
}

// String returns the string representation corresponding to the token.
func (t Token) String() string {
	if int(t) >= len(tokenNames) {
		return "ILLEGAL"
	}

	name := tokenNames[t]
	if name == "" {
		return "ILLEGAL"
	}
	return name
}

// GoString returns the %#v format of t.
func (t Token) GoString() string { return t.String() }

// IsKeyword returns true if the token corresponds to a keyword.
func (t Token) IsKeyword() bool { return t > keywordBeg && t < keywordEnd }

// IsLiteral returns true if the token corresponds to a literal token or
// identifier.
func (t Token) IsLiteral() bool { return t > literalBeg && t < literalEnd }

// IsOperator returns true if the token corresponds to an operator or
// delimiter.
func (t Token) IsOperator() bool { return t > operatorBeg && t < operatorEnd }

// BinaryPrecedence returns the operator precedence of the binary operator t.
// If t is not a binary operator, the result is LowestPrecedence.
func (t Token) BinaryPrecedence() int {
	switch t {
	case OR:
		return 1
	case AND:
		return 2
	case EQ, NEQ, LT, LTE, GT, GTE:
		return 3
	case ADD, SUB:
		return 4
	case MUL, DIV, MOD:
		return 5
	case POW:
		return 6
	}

	return LowestPrecedence
}

// Levels of precedence for operator tokens.
const (
	LowestPrecedence  = 0 // non-operators
	UnaryPrecedence   = 7
	HighestPrecedence = 8
)
