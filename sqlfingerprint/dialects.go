package sqlfingerprint

import "strings"

// mysqlFunctions identifies built-ins emitted as syntax tokens in DIGEST_TEXT.
// Ordinary function identifiers (for example concat) retain their spelling.
var mysqlFunctions = wordSet(`
adddate bit_and bit_or bit_xor cast coalesce count curdate curtime date_add date_sub
extract group_concat json_arrayagg json_objectagg max mid min now position
session_user std stddev stddev_pop stddev_samp subdate substr substring sum sysdate
system_user trim variance var_pop var_samp avg
`)

// mysqlKeywords supplements the shared syntax set with MySQL keyword tokens.
// COMMENT is nonreserved, so it can name a column while still being emitted as
// an uppercase keyword in DIGEST_TEXT. Quoted names retain their identifier kind.
// Source: https://github.com/mysql/mysql-server/blob/8.4/sql/lex.h
var mysqlKeywords = wordSet("comment")

// postgresTypes limits cast squashing to built-in type names. A user-defined
// cast may execute a function and must not be mistaken for a constant expression.
var postgresTypes = wordSet(`
bigint int8 smallint int2 integer int int4 real float4 float8 double precision
numeric decimal bool boolean text varchar character varying char bpchar name
date time timestamp interval uuid bytea bit varbit
`)

// continuationCommand distinguishes DML within INSERT/MERGE/CTE statements and
// SELECT FOR UPDATE from a second command lacking a statement separator.
func continuationCommand(tokens []token, i int, first string) bool {
	if first == mergeCommand {
		return true
	}
	if strings.EqualFold(tokens[i].text, updateCommand) {
		if i > 0 && strings.EqualFold(tokens[i-1].text, "for") {
			return true
		}
		if first == insertCommand {
			return i > 0 && (strings.EqualFold(tokens[i-1].text, "do") || strings.EqualFold(tokens[i-1].text, "key"))
		}
	}
	if first == withCommand {
		for _, t := range tokens[1:i] {
			if t.kind == word && (strings.EqualFold(t.text, updateCommand) || strings.EqualFold(t.text, deleteCommand) || strings.EqualFold(t.text, insertCommand)) {
				return false
			}
		}
		return true
	}
	return false
}

// foldPostgreSQLName folds ASCII identifier bytes without changing multibyte
// letters. PostgreSQL doesn't apply Unicode case folding to UTF-8 identifiers.
func foldPostgreSQLName(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, name)
}

// appendSingletonIN mirrors PostgreSQL's scalar comparison for IN with one
// literal. Statistics can retain either spelling as their representative text.
func appendSingletonIN(out []token, item token) []token {
	out = out[:len(out)-1]
	op := "="
	if len(out) > 0 && out[len(out)-1] == (token{word, "not"}) {
		out = out[:len(out)-1]
		op = "<>"
	}
	return append(out, token{symbol, op}, item)
}

// appendArray normalizes PostgreSQL 18's constant ARRAY list markers. Ordinary
// array subscripts and expressions preserve every element and delimiter.
func appendArray(out, inside []token, dialect Dialect, options Options) []token {
	if dialect == PostgreSQL && options.PostgreSQLVersion >= 18 && len(out) > 0 && out[len(out)-1] == (token{word, "array"}) && postgresConstantList(inside) {
		inside = []token{{valueList, "array"}}
	}
	out = append(out, token{symbol, "["})
	out = append(out, inside...)
	return append(out, token{symbol, "]"})
}

// postgresConstantList recognizes two or more constants, optionally decorated
// with built-in casts, or PostgreSQL's exact commented-out list marker.
func postgresConstantList(tokens []token) bool {
	if len(tokens) == 2 && tokens[0].kind == value && tokens[1].kind == listComment {
		return true
	}
	count := 0
	for len(tokens) > 0 {
		if tokens[0].kind != value {
			return false
		}
		tokens = tokens[1:]
		if len(tokens) > 0 && tokens[0] == (token{symbol, "::"}) {
			end := 1
			for end < len(tokens) && tokens[end] != (token{symbol, ","}) {
				if !postgresTypes[strings.ToLower(tokens[end].text)] {
					return false
				}
				end++
			}
			if end == 1 {
				return false
			}
			tokens = tokens[end:]
		}
		count++
		if len(tokens) == 0 {
			return count >= 2
		}
		if tokens[0] != (token{symbol, ","}) {
			return false
		}
		tokens = tokens[1:]
	}
	return false
}
