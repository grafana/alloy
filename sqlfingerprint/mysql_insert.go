package sqlfingerprint

import (
	"slices"
	"strings"
)

// normalizeMySQLInsertColumns groups simple constant-row INSERTs by their column
// set. Values have already collapsed to row markers, so their positions no longer
// carry information. Expressions and trailing clauses keep their original order.
func normalizeMySQLInsertColumns(tokens []token) []token {
	start := mysqlInsertColumnStart(tokens)
	if start < 0 {
		return tokens
	}
	columns, end := mysqlInsertColumns(tokens, start)
	if len(columns) < 2 || !mysqlInsertConstantRows(tokens[end:]) {
		return tokens
	}
	slices.SortFunc(columns, compareMySQLInsertColumns)
	for i, column := range columns {
		tokens[start+2*i] = column
	}
	return tokens
}

// mysqlInsertColumnStart recognizes INSERT [IGNORE] [INTO] table (columns),
// including a database-qualified table. Other INSERT forms keep their order.
func mysqlInsertColumnStart(tokens []token) int {
	if len(tokens) == 0 || tokens[0] != (token{word, insertCommand}) {
		return -1
	}
	i := 1
	for _, optional := range []string{"ignore", "into"} {
		if i < len(tokens) && tokens[i] == (token{word, optional}) {
			i++
		}
	}
	if i >= len(tokens) || !mysqlInsertName(tokens[i]) {
		return -1
	}
	i++
	if i+1 < len(tokens) && tokens[i] == (token{symbol, "."}) && mysqlInsertName(tokens[i+1]) {
		i += 2
	}
	if i >= len(tokens) || tokens[i] != (token{symbol, "("}) {
		return -1
	}
	return i + 1
}

// mysqlInsertColumns reads a nonempty list of distinct simple names and returns
// the position after its closing parenthesis. Invalid lists are left unchanged.
func mysqlInsertColumns(tokens []token, start int) ([]token, int) {
	var columns []token
	seen := make(map[string]bool)
	for i := start; i+1 < len(tokens); i += 2 {
		column := tokens[i]
		if !mysqlInsertName(column) || seen[column.text] {
			return nil, 0
		}
		seen[column.text] = true
		columns = append(columns, column)
		switch tokens[i+1] {
		case token{symbol, ")"}:
			return columns, i + 2
		case token{symbol, ","}:
			continue
		default:
			return nil, 0
		}
	}
	return nil, 0
}

// mysqlInsertName accepts identifiers and nonreserved keyword tokens such as
// COMMENT, which MySQL can emit for an unquoted table or column name.
func mysqlInsertName(t token) bool {
	return t.kind == identifier || t.kind == word
}

// mysqlInsertConstantRows requires the entire VALUES tail to be one normalized
// multi-value row marker. Repeated-row markers qualify but remain distinct from
// single rows. Expressions, DEFAULT, row aliases, and update clauses don't qualify.
func mysqlInsertConstantRows(tokens []token) bool {
	if len(tokens) != 2 || (tokens[0] != (token{word, "values"}) && tokens[0] != (token{word, "value"})) {
		return false
	}
	return tokens[1].kind == rowMultiple || tokens[1].kind == rowMultipleList
}

// compareMySQLInsertColumns gives unique column names a deterministic order
// without changing their spelling or their identifier-versus-keyword token kind.
func compareMySQLInsertColumns(left, right token) int {
	return strings.Compare(left.text, right.text)
}
