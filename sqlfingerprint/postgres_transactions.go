package sqlfingerprint

import "strings"

// normalizePostgresTransaction recognizes complete transaction commands before
// the generic DML shape checks. Syntax words are folded only in this context,
// so adding transaction support cannot reclassify identifiers in other queries.
func normalizePostgresTransaction(tokens []token) ([]token, Reason, bool) {
	if len(tokens) == 0 || tokens[0].kind != word {
		return nil, "", false
	}
	p := transactionParser{tokens: tokens, pos: 1}
	first := strings.ToLower(tokens[0].text)
	valid := false
	switch first {
	case beginCommand, "start":
		if first == "start" {
			if !p.take("transaction") {
				return nil, InvalidSQL, true
			}
		} else {
			p.work()
		}
		valid = p.modes()
	case "commit", "end", "rollback", "abort":
		if p.take("prepared") {
			return nil, Unsupported, true
		}
		p.work()
		if first == "rollback" && p.take("to") {
			p.take("savepoint")
			valid = p.name()
		} else {
			valid = true
			if p.take("and") {
				p.take("no")
				valid = p.take("chain")
			}
		}
	case "savepoint":
		valid = p.name()
	case "release":
		p.take("savepoint")
		valid = p.name()
	default:
		return nil, "", false
	}
	if !valid || p.pos != len(tokens) {
		return nil, InvalidSQL, true
	}
	out := append([]token(nil), tokens...)
	for i := range out {
		if i == p.nameIndex && p.nameIndex > 0 {
			if out[i].kind == word {
				out[i] = token{identifier, foldPostgreSQLName(out[i].text)}
			}
		} else if out[i].kind == word {
			out[i].text = strings.ToLower(out[i].text)
		}
	}
	return out, "", true
}

// transactionParser retains original spelling and option order in its input.
type transactionParser struct {
	tokens    []token
	pos       int
	nameIndex int
}

func (p *transactionParser) take(keyword string) bool {
	if p.pos < len(p.tokens) && p.tokens[p.pos].kind == word && strings.EqualFold(p.tokens[p.pos].text, keyword) {
		p.pos++
		return true
	}
	return false
}

func (p *transactionParser) work() {
	if !p.take("work") {
		p.take("transaction")
	}
}

func (p *transactionParser) name() bool {
	if p.pos == len(p.tokens) || (p.tokens[p.pos].kind != word && p.tokens[p.pos].kind != identifier) {
		return false
	}
	p.nameIndex = p.pos
	p.pos++
	return true
}

// modes accepts PostgreSQL's optional commas between transaction modes.
func (p *transactionParser) modes() bool {
	for p.pos < len(p.tokens) {
		switch {
		case p.take("isolation"):
			if !p.take("level") {
				return false
			}
			switch {
			case p.take("serializable"):
			case p.take("repeatable"):
				if !p.take("read") {
					return false
				}
			case p.take("read"):
				if !p.take("committed") && !p.take("uncommitted") {
					return false
				}
			default:
				return false
			}
		case p.take("read"):
			if !p.take("write") && !p.take("only") {
				return false
			}
		case p.take("not"):
			if !p.take("deferrable") {
				return false
			}
		case p.take("deferrable"):
		default:
			return false
		}
		if p.pos < len(p.tokens) && p.tokens[p.pos] == (token{symbol, ","}) {
			p.pos++
			if p.pos == len(p.tokens) {
				return false
			}
		}
	}
	return true
}
