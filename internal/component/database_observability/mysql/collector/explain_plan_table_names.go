package collector

import (
	"strconv"
	"strings"
)

// rewrittenQueryNoteCode is the code of the note that `SHOW WARNINGS` returns
// after `EXPLAIN`, holding the query as the optimizer rewrote it. Every table
// in it is written as `schema`.`table`, followed by its alias when it has one.
const rewrittenQueryNoteCode = 1003

// planTableKey identifies a table the way an explain plan names it: by the
// alias (or the table name when there is no alias) within one query block.
type planTableKey struct {
	selectID int
	name     string
}

// tableRef is a table's real schema and name.
type tableRef struct {
	schema string
	table  string
}

// planTableKind says what a name in an explain plan refers to.
type planTableKind int

const (
	// planTableUnknown means the name could not be resolved: the rewritten
	// query was not available, or the same name refers to more than one table.
	planTableUnknown planTableKind = iota
	planTableReal
	// planTableDerived is a derived table or a materialized CTE, which no index
	// could change.
	planTableDerived
)

// planTableNames maps the names an explain plan uses to the tables behind them.
type planTableNames struct {
	refs      map[planTableKey]tableRef
	derived   map[planTableKey]struct{}
	ambiguous map[planTableKey]struct{}
}

// resolve returns what the name an explain plan uses in a query block refers to.
// When the query block holds no such name, a name that is unique across the
// whole query is used instead.
func (n planTableNames) resolve(selectID int, name string) (tableRef, planTableKind) {
	key := planTableKey{selectID: selectID, name: name}
	if _, ok := n.ambiguous[key]; ok {
		return tableRef{}, planTableUnknown
	}
	if ref, ok := n.refs[key]; ok {
		return ref, planTableReal
	}
	if _, ok := n.derived[key]; ok {
		return tableRef{}, planTableDerived
	}

	var (
		found tableRef
		count int
	)
	for k, ref := range n.refs {
		if k.name == name {
			found = ref
			count++
		}
	}
	for k := range n.derived {
		if k.name == name {
			count++
		}
	}
	for k := range n.ambiguous {
		if k.name == name {
			count++
		}
	}
	if count == 1 && found != (tableRef{}) {
		return found, planTableReal
	}
	return tableRef{}, planTableUnknown
}

func (n *planTableNames) add(key planTableKey, ref tableRef) {
	if _, ok := n.ambiguous[key]; ok {
		return
	}
	if existing, ok := n.refs[key]; ok && existing != ref {
		delete(n.refs, key)
		n.ambiguous[key] = struct{}{}
		return
	}
	n.refs[key] = ref
}

// parseRewrittenQueryTables reads the tables of the rewritten query in a
// `SHOW WARNINGS` note. It only looks at the table references after FROM and
// JOIN, so column references, conditions and literals are never mistaken for
// tables.
func parseRewrittenQueryTables(note string) planTableNames {
	names := planTableNames{
		refs:      make(map[planTableKey]tableRef),
		derived:   make(map[planTableKey]struct{}),
		ambiguous: make(map[planTableKey]struct{}),
	}

	type frame struct {
		selectID int
		// inFrom is set from FROM until the clause that ends the table list.
		inFrom bool
		// expectTable is set where a table reference can start.
		expectTable bool
		// inOn is set inside a join condition, where commas don't start tables.
		inOn bool
		// derivedOpen is set when this frame's table reference is a
		// parenthesized subquery, whose alias follows the closing parenthesis.
		derivedOpen bool
	}

	tokens := tokenizeRewrittenQuery(note)
	stack := []frame{{}}
	pendingSelect := 0

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		top := &stack[len(stack)-1]

		switch tok.kind {
		case tokenSelectMarker:
			pendingSelect = tok.selectID

		case tokenPunct:
			switch tok.text {
			case "(":
				child := frame{selectID: top.selectID, inFrom: top.expectTable, expectTable: top.expectTable}
				top.derivedOpen = top.expectTable
				top.expectTable = false
				stack = append(stack, child)
			case ")":
				if len(stack) > 1 {
					stack = stack[:len(stack)-1]
				}
				parent := &stack[len(stack)-1]
				if parent.derivedOpen {
					parent.derivedOpen = false
					// A derived table: `(select ...) alias`.
					if i+1 < len(tokens) && tokens[i+1].kind == tokenIdent && !followedByDot(tokens, i+1) {
						names.derived[planTableKey{selectID: parent.selectID, name: tokens[i+1].text}] = struct{}{}
						i++
					}
				}
			case ",":
				if top.inFrom && !top.inOn {
					top.expectTable = true
				}
			}

		case tokenWord:
			switch strings.ToLower(tok.text) {
			case "select":
				if pendingSelect != 0 {
					top.selectID = pendingSelect
					pendingSelect = 0
				}
				top.inFrom, top.expectTable, top.inOn = false, false, false
			case "from":
				top.inFrom, top.expectTable, top.inOn = true, true, false
			case "join", "straight_join":
				if top.inFrom {
					top.expectTable, top.inOn = true, false
				}
			case "on", "using":
				if top.inFrom {
					top.expectTable, top.inOn = false, true
				}
			case "where", "group", "having", "order", "limit", "window", "union", "except", "intersect", "for", "into", "lock":
				top.inFrom, top.expectTable, top.inOn = false, false, false
			}

		case tokenIdent:
			if !top.expectTable {
				continue
			}
			top.expectTable = false

			var ref tableRef
			if followedByDot(tokens, i) && i+2 < len(tokens) && tokens[i+2].kind == tokenIdent {
				ref = tableRef{schema: tok.text, table: tokens[i+2].text}
				i += 2
			} else {
				// A single name is a CTE or derived table defined elsewhere.
				name := tok.text
				if i+1 < len(tokens) && tokens[i+1].kind == tokenIdent && !followedByDot(tokens, i+1) {
					name = tokens[i+1].text
					i++
				}
				names.derived[planTableKey{selectID: top.selectID, name: name}] = struct{}{}
				continue
			}

			name := ref.table
			if i+1 < len(tokens) && tokens[i+1].kind == tokenIdent && !followedByDot(tokens, i+1) {
				name = tokens[i+1].text
				i++
			}
			names.add(planTableKey{selectID: top.selectID, name: name}, ref)
		}
	}

	return names
}

func followedByDot(tokens []rewrittenQueryToken, i int) bool {
	return i+1 < len(tokens) && tokens[i+1].kind == tokenPunct && tokens[i+1].text == "."
}

type rewrittenQueryTokenKind int

const (
	tokenWord rewrittenQueryTokenKind = iota
	tokenIdent
	tokenPunct
	tokenSelectMarker
)

type rewrittenQueryToken struct {
	kind     rewrittenQueryTokenKind
	text     string
	selectID int
}

// tokenizeRewrittenQuery splits a rewritten query into words, backtick-quoted
// identifiers, punctuation and `/* select#N */` markers. String literals and
// other comments are dropped.
func tokenizeRewrittenQuery(s string) []rewrittenQueryToken {
	var tokens []rewrittenQueryToken
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++

		case c == '`':
			var b strings.Builder
			j := i + 1
			for j < len(s) {
				if s[j] == '`' {
					if j+1 < len(s) && s[j+1] == '`' {
						b.WriteByte('`')
						j += 2
						continue
					}
					break
				}
				b.WriteByte(s[j])
				j++
			}
			tokens = append(tokens, rewrittenQueryToken{kind: tokenIdent, text: b.String()})
			i = j + 1

		case c == '\'' || c == '"':
			j := i + 1
			for j < len(s) {
				if s[j] == '\\' {
					j += 2
					continue
				}
				if s[j] == c {
					if j+1 < len(s) && s[j+1] == c {
						j += 2
						continue
					}
					break
				}
				j++
			}
			i = j + 1

		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return tokens
			}
			body := strings.TrimSpace(s[i+2 : i+2+end])
			if n, ok := strings.CutPrefix(body, "select#"); ok {
				if id, err := strconv.Atoi(n); err == nil {
					tokens = append(tokens, rewrittenQueryToken{kind: tokenSelectMarker, selectID: id})
				}
			}
			i += end + 4

		case isWordByte(c):
			j := i
			for j < len(s) && isWordByte(s[j]) {
				j++
			}
			tokens = append(tokens, rewrittenQueryToken{kind: tokenWord, text: s[i:j]})
			i = j

		default:
			tokens = append(tokens, rewrittenQueryToken{kind: tokenPunct, text: string(c)})
			i++
		}
	}
	return tokens
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
