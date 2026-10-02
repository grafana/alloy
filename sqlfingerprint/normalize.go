package sqlfingerprint

import "strings"

const (
	selectCommand = "select"
	insertCommand = "insert"
	updateCommand = "update"
	deleteCommand = "delete"
	mergeCommand  = "merge"
	withCommand   = "with"
	beginCommand  = "begin"
)

// keywords folds SQL syntax without lowercasing case-sensitive object names.
var keywords = wordSet(`
all alter and any array as asc between both by call case cast collate conflict cross
current current_date current_time current_timestamp database default delete desc
distinct do drop else end escape except exclude exists explain false fetch filter
first following for force foreign from full group groups having ignore ilike in
index inner insert intersect interval into is join key lateral leading left like
limit lock matched merge natural next not nothing null nulls offset on only or
order outer over partition preceding primary range recursive regexp replace
returning right row rows select set similar some table then ties top trailing true
truncate unbounded union unique update using value values when where window with
within xor duplicate materialized option output pivot unpivot recompile readpast
nolock updlock holdlock readcommitted rowlock skip locked wait nowait binary div mod
`)

// wordSet constructs a read-only lookup table for syntax classifications.
func wordSet(words string) map[string]bool {
	set := make(map[string]bool)
	for _, s := range strings.Fields(words) {
		set[s] = true
	}
	return set
}

// normalize accepts query tokens and native digest list markers. It checks
// statement structure before applying the versioned equivalence rules.
func normalize(tokens []token, dialect Dialect, options Options) ([]token, Reason) {
	if dialect == PostgreSQL {
		tokens = stripPostgresObfuscatedComment(tokens)
		tokens = normalizePostgresParameters(tokens)
		if normalized, reason, handled := normalizePostgresTransaction(tokens); handled {
			return normalized, reason
		}
	}
	if dialect == SQLServer {
		tokens = stripParameterDeclaration(tokens)
	}
	if reason := validateShape(tokens); reason != "" {
		return nil, reason
	}
	tokens = normalizeAtoms(tokens, dialect)
	result, _, reason := normalizeGroups(tokens, 0, dialect, options)
	if reason == "" && dialect == MySQL {
		result = normalizeMySQLInsertColumns(result)
	}
	return result, reason
}

// unsafeBatch detects procedural contexts where semicolons are not reliable
// boundaries. PostgreSQL transaction starts have safe statement boundaries.
func unsafeBatch(tokens []token, dialect Dialect) bool {
	if len(tokens) == 0 || tokens[0].kind != word {
		return false
	}
	switch strings.ToLower(tokens[0].text) {
	case beginCommand:
		if dialect == PostgreSQL {
			_, reason, handled := normalizePostgresTransaction(stripPostgresObfuscatedComment(tokens))
			return !handled || reason != ""
		}
		return true
	case "declare", "do", "if", "while", "delimiter":
		return true
	case "create", "alter":
		for _, t := range tokens {
			if t.kind == word {
				switch strings.ToLower(t.text) {
				case "procedure", "proc", "function", "trigger", "event":
					return true
				}
			}
		}
	}
	return false
}

// stripParameterDeclaration handles SQL Server cached text such as
// (@p int)SELECT ... . Ordinary parenthesized expressions are left intact.
func stripParameterDeclaration(tokens []token) []token {
	if len(tokens) < 4 || tokens[0].text != "(" || tokens[1].kind != parameter {
		return tokens
	}
	depth := 0
	for i, t := range tokens {
		if t.kind != symbol {
			continue
		}
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return tokens[i+1:]
			}
		}
	}
	return nil
}

// validateShape rejects unsupported families and incomplete tails. This
// structural parser does not perform server or catalog-aware SQL validation.
func validateShape(tokens []token) Reason {
	if len(tokens) < 2 || tokens[0].kind != word {
		return InvalidSQL
	}
	first := strings.ToLower(tokens[0].text)
	switch first {
	case selectCommand, insertCommand, updateCommand, deleteCommand, mergeCommand, withCommand, "values":
	default:
		return Unsupported
	}
	if reason := validateBoundaries(tokens, first); reason != "" {
		return reason
	}
	last := tokens[len(tokens)-1]
	if last.kind == symbol && last.text != ")" && last.text != "]" && last.text != "*" {
		return InvalidSQL
	}
	if last.kind == word && strings.Contains(" from where and or set into join on as by having limit offset union select ", " "+strings.ToLower(last.text)+" ") {
		return InvalidSQL
	}
	if first == selectCommand && tokens[1].kind == word && strings.EqualFold(tokens[1].text, "from") {
		return InvalidSQL
	}
	return ""
}

// validateBoundaries balances delimiters and rejects obvious semicolon-free
// batches. Supported batches must have explicit, unambiguous statement separators.
func validateBoundaries(tokens []token, first string) Reason {
	var stack []string
	selects := 0
	for i, t := range tokens {
		if t.kind == symbol {
			switch t.text {
			case "(", "[":
				stack = append(stack, t.text)
				if len(stack) > 128 {
					return Limit
				}
			case ")", "]":
				if len(stack) == 0 || (t.text == ")") != (stack[len(stack)-1] == "(") {
					return InvalidSQL
				}
				stack = stack[:len(stack)-1]
			}
		}
		if len(stack) == 0 && t.kind == word {
			w := strings.ToLower(t.text)
			if w == "go" || w == beginCommand || w == "declare" || w == "exec" || w == "execute" {
				return Unsupported
			}
			if w == selectCommand {
				selects++
				if selects > 1 && !afterSetOperator(tokens, i) {
					return Unsupported
				}
			}
			if i > 0 && (w == updateCommand || w == deleteCommand || w == insertCommand || w == mergeCommand) && !continuationCommand(tokens, i, first) {
				return Unsupported
			}
		}
	}
	if len(stack) != 0 {
		return InvalidSQL
	}
	return ""
}

// afterSetOperator permits SELECT following UNION, INTERSECT, or EXCEPT.
func afterSetOperator(tokens []token, i int) bool {
	if i > 0 && tokens[i-1].kind == word && (strings.EqualFold(tokens[i-1].text, "all") || strings.EqualFold(tokens[i-1].text, "distinct")) {
		i--
	}
	return i > 0 && tokens[i-1].kind == word && strings.Contains(" union intersect except ", " "+strings.ToLower(tokens[i-1].text)+" ")
}

// normalizeAtoms erases literal values, parameter names, and unary numeric
// signs while preserving binary operators and schema-qualified identifiers.
func normalizeAtoms(tokens []token, dialect Dialect) []token {
	out := make([]token, 0, len(tokens))
	for i, t := range tokens {
		if t.kind == symbol && (t.text == "+" || t.text == "-") && i+1 < len(tokens) && tokens[i+1].kind == number && expectsValue(out) {
			continue
		}
		if t.kind == number || t.kind == parameter {
			t.kind = value
		}
		if t.kind == word {
			t = normalizeWord(t.text, dialect, out)
			if dialect == MySQL && i+1 < len(tokens) && tokens[i+1] == (token{symbol, "("}) && mysqlFunctions[strings.ToLower(tokens[i].text)] {
				t = token{word, strings.ToLower(tokens[i].text)}
			}
		}
		out = append(out, t)
	}
	return out
}

// normalizeWord preserves NULL/boolean predicates, where the word is SQL
// syntax, but erases literal expressions such as SELECT NULL.
func normalizeWord(name string, dialect Dialect, previous []token) token {
	lower := strings.ToLower(name)
	if lower == "null" || (dialect == PostgreSQL && (lower == "true" || lower == "false")) {
		i := len(previous) - 1
		if i >= 0 && previous[i] == (token{word, "not"}) {
			i--
		}
		if i < 0 || previous[i] != (token{word, "is"}) {
			return token{value, "?"}
		}
	}
	if keywords[lower] || (dialect == MySQL && mysqlKeywords[lower]) {
		return token{word, lower}
	}
	if dialect == PostgreSQL {
		name = foldPostgreSQLName(name)
	}
	return token{identifier, name}
}

// expectsValue distinguishes a signed literal from binary subtraction/addition.
func expectsValue(tokens []token) bool {
	if len(tokens) == 0 {
		return true
	}
	t := tokens[len(tokens)-1]
	if t.kind == symbol {
		return t.text != ")" && t.text != "]" && t.text != "."
	}
	return t.kind == word && strings.Contains(" select where and or xor not then else when between like in values set limit offset top returning by ", " "+t.text+" ")
}

// normalizeGroups walks parentheses from the inside out. Recursion depth is
// bounded by validateBoundaries before entering this function.
func normalizeGroups(tokens []token, start int, dialect Dialect, options Options) ([]token, int, Reason) {
	var out []token
	for i := start; i < len(tokens); i++ {
		t := tokens[i]
		if t == (token{symbol, ")"}) || t == (token{symbol, "]"}) {
			return reduceLists(out, dialect), i, ""
		}
		if t == (token{symbol, "("}) || t == (token{symbol, "["}) {
			inside, end, reason := normalizeGroups(tokens, i+1, dialect, options)
			if reason != "" {
				return nil, 0, reason
			}
			if t.text == "[" {
				out = appendArray(out, inside, dialect, options)
			} else {
				out = appendGroup(out, inside, dialect, options)
			}
			i = end
		} else {
			out = append(out, t)
		}
	}
	out = reduceLists(out, dialect)
	for _, t := range out {
		if t.kind == ellipsis || t.kind == listComment {
			return nil, 0, Truncated
		}
	}
	return out, len(tokens), ""
}

// appendGroup collapses only recognizable native list shapes. Subqueries,
// casts, column references, and function expressions keep their structure.
func appendGroup(out, inside []token, dialect Dialect, options Options) []token {
	in := len(out) > 0 && out[len(out)-1] == (token{word, "in"})
	if in && dialect == PostgreSQL {
		if len(inside) == 1 && inside[0].kind == value {
			return appendSingletonIN(out, inside[0])
		}
		if options.PostgreSQLVersion >= 18 && postgresConstantList(inside) {
			return append(out, token{valueList, "in"})
		}
	}
	if in && dialect == MySQL && constantList(inside) {
		return append(out, token{valueList, "in"})
	}
	if dialect == MySQL && len(inside) == 1 {
		switch inside[0].kind {
		case value:
			return append(out, token{rowSingle, "row"})
		case valueList, ellipsis:
			return append(out, token{rowMultiple, "row"})
		}
	}
	out = append(out, token{symbol, "("})
	out = append(out, inside...)
	return append(out, token{symbol, ")"})
}

// constantList recognizes literals and exact database list markers, without
// conflating lists of expressions with lists of constants.
func constantList(tokens []token) bool {
	if len(tokens) == 1 && (tokens[0].kind == ellipsis || tokens[0].kind == valueList) {
		return true
	}
	if len(tokens) == 2 && tokens[0].kind == value && tokens[1].kind == listComment {
		return true
	}
	if len(tokens)%2 != 1 {
		return false
	}
	for i, t := range tokens {
		if i%2 == 0 && t.kind != value {
			return false
		}
		if i%2 == 1 && t != (token{symbol, ","}) {
			return false
		}
	}
	return len(tokens) > 0
}

// reduceLists uses a stack to match MySQL's value and repeated-row markers in
// linear time, including very large INSERT batches and IN lists.
func reduceLists(tokens []token, dialect Dialect) []token {
	if dialect != MySQL {
		return tokens
	}
	out := make([]token, 0, len(tokens))
	for _, t := range tokens {
		if len(out) > 0 && t.kind == listComment {
			if kind, ok := repeatedKind(out[len(out)-1].kind); ok {
				out[len(out)-1] = token{kind, "list"}
				continue
			}
		}
		if len(out) >= 2 && out[len(out)-1] == (token{symbol, ","}) {
			if kind, ok := combineList(out[len(out)-2].kind, t.kind); ok {
				out = out[:len(out)-1]
				out[len(out)-1] = token{kind, "list"}
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// repeatedKind retains the single-column versus multi-column row distinction
// that MySQL preserves in DIGEST_TEXT.
func repeatedKind(kind tokenKind) (tokenKind, bool) {
	switch kind {
	case rowSingle, rowSingleList:
		return rowSingleList, true
	case rowMultiple, rowMultipleList:
		return rowMultipleList, true
	default:
		return 0, false
	}
}

// combineList joins generic values or compatible rows, accepting MySQL's
// explicit "?, ..." marker as well as unnormalized literal lists.
func combineList(left, right tokenKind) (tokenKind, bool) {
	if (left == value || left == valueList) && (right == value || right == valueList || right == ellipsis) {
		return valueList, true
	}
	l, lok := repeatedKind(left)
	r, rok := repeatedKind(right)
	return l, lok && rok && l == r
}
