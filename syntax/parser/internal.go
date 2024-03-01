package parser

import (
	"fmt"
	"strings"

	"github.com/grafana/river/ast"
	"github.com/grafana/river/diag"
	"github.com/grafana/river/scanner"
	"github.com/grafana/river/token"
)

// parser implements the River parser.
//
// It is only safe for callers to use exported methods as entrypoints for
// parsing.
//
// Each Parse* and parse* method will describe the EBNF grammar being used for
// parsing that non-terminal. The EBNF grammar will be written as LL(1) and
// should directly represent the code.
//
// The parser will continue on encountering errors to allow a more complete
// list of errors to be returned to the user. The resulting AST should be
// discarded if errors were encountered during parsing.
type parser struct {
	file     *token.File
	diags    diag.Diagnostics
	scanner  *scanner.Scanner
	comments []ast.CommentGroup

	pos token.Pos   // Current token position
	tok token.Token // Current token
	lit string      // Current token literal

	// Position of the last error written. Two parse errors on the same line are
	// ignored.
	lastError token.Position
}

// newParser creates a new parser which will parse the provided src.
func newParser(filename string, src []byte) *parser {
	file := token.NewFile(filename)

	p := &parser{
		file: file,
	}

	p.scanner = scanner.New(file, src, func(pos token.Pos, msg string) {
		p.diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: file.PositionFor(pos),
			Message:  msg,
		})
	}, scanner.IncludeComments)

	p.next()
	return p
}

// next advances the parser to the next non-comment token.
func (p *parser) next() {
	p.next0()

	for p.tok == token.COMMENT {
		p.consumeCommentGroup()
	}
}

// next0 advances the parser to the next token. next0 should not be used
// directly by parse methods; call next instead.
func (p *parser) next0() { p.pos, p.tok, p.lit = p.scanner.Scan() }

// consumeCommentGroup consumes a group of adjacent comments, adding it to p's
// comment list.
func (p *parser) consumeCommentGroup() {
	var list []*ast.Comment

	endline := p.pos.Position().Line
	for p.tok == token.COMMENT && p.pos.Position().Line <= endline+1 {
		var comment *ast.Comment
		comment, endline = p.consumeComment()
		list = append(list, comment)
	}

	p.comments = append(p.comments, ast.CommentGroup(list))
}

// consumeComment consumes a comment and returns it with the line number it
// ends on.
func (p *parser) consumeComment() (comment *ast.Comment, endline int) {
	endline = p.pos.Position().Line

	if p.lit[1] == '*' {
		// Block comments may end on a different line than where they start. Scan
		// the comment for newlines and adjust endline accordingly.
		//
		// NOTE: don't use range here, since range will unnecessarily decode
		// Unicode code points and slow down the parser.
		for i := 0; i < len(p.lit); i++ {
			if p.lit[i] == '\n' {
				endline++
			}
		}
	}

	comment = &ast.Comment{StartPos: p.pos, Text: p.lit}
	p.next0()
	return
}

// advance consumes tokens up to (but not including) the specified token.
// advance will stop consuming tokens if EOF is reached before to.
func (p *parser) advance(to token.Token) {
	for p.tok != token.EOF {
		if p.tok == to {
			return
		}
		p.next()
	}
}

// advanceAny consumes tokens up to (but not including) any of the tokens in
// the to set.
func (p *parser) advanceAny(to map[token.Token]struct{}) {
	for p.tok != token.EOF {
		if _, inSet := to[p.tok]; inSet {
			return
		}
		p.next()
	}
}

// expect consumes the next token. It records an error if the consumed token
// was not t.
func (p *parser) expect(t token.Token) (pos token.Pos, tok token.Token, lit string) {
	pos, tok, lit = p.pos, p.tok, p.lit
	if tok != t {
		p.addErrorf("expected %s, got %s", t, p.tok)
	}
	p.next()
	return
}

func (p *parser) addErrorf(format string, args ...interface{}) {
	pos := p.file.PositionFor(p.pos)

	// Ignore errors which occur on the same line.
	if p.lastError.Line == pos.Line {
		return
	}
	p.lastError = pos

	p.diags.Add(diag.Diagnostic{
		Severity: diag.SeverityLevelError,
		StartPos: pos,
		Message:  fmt.Sprintf(format, args...),
	})
}

// ParseFile parses an entire file.
//
//	File = Body
func (p *parser) ParseFile() *ast.File {
	body := p.parseBody(token.EOF)

	return &ast.File{
		Name:     p.file.Name(),
		Body:     body,
		Comments: p.comments,
	}
}

// parseBody parses a series of statements up to and including the "until"
// token, which terminates the body.
//
//	Body = [ Statement { terminator Statement } ]
func (p *parser) parseBody(until token.Token) ast.Body {
	var body ast.Body

	for p.tok != until && p.tok != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}

		if p.tok == until {
			break
		}

		if p.tok != token.TERMINATOR {
			p.addErrorf("expected %s, got %s", token.TERMINATOR, p.tok)
			p.consumeStatement()
		}
		p.next()
	}

	return body
}

// consumeStatement consumes tokens for the remainder of a statement (i.e., up
// to but not including a terminator). consumeStatement will keep track of the
// number of {}, [], and () pairs, only returning after the count of pairs is
// <= 0.
func (p *parser) consumeStatement() {
	var curlyPairs, brackPairs, parenPairs int

	for p.tok != token.EOF {
		switch p.tok {
		case token.LCURLY:
			curlyPairs++
		case token.RCURLY:
			curlyPairs--
		case token.LBRACK:
			brackPairs++
		case token.RBRACK:
			brackPairs--
		case token.LPAREN:
			parenPairs++
		case token.RPAREN:
			parenPairs--
		}

		if p.tok == token.TERMINATOR {
			// Only return after we've consumed all pairs. It's possible for pairs to
			// be less than zero if our statement started in a surrounding pair.
			if curlyPairs <= 0 && brackPairs <= 0 && parenPairs <= 0 {
				return
			}
		}

		p.next()
	}
}

// parseStatement parses an individual statement within a body.
//
//	Statement = Attribute | Block
//	Attribute = identifier "=" Expression
//	Block     = BlockName "{" Body "}"
func (p *parser) parseStatement() ast.Stmt {
	blockName := p.parseBlockName()
	if blockName == nil {
		// parseBlockName failed; skip to the next identifier which would start a
		// new Statement.
		p.advance(token.IDENT)
		return nil
	}

	// p.tok is now the first token after the identifier in the attribute or
	// block name.
	switch p.tok {
	case token.ASSIGN: // Attribute
		p.next() // Consume "="

		if len(blockName.Fragments) != 1 {
			attrName := strings.Join(blockName.Fragments, ".")
			p.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: blockName.Start.Position(),
				EndPos:   blockName.Start.Add(len(attrName) - 1).Position(),
				Message:  `attribute names may only consist of a single identifier with no "."`,
			})
		} else if blockName.LabelPos != token.NoPos {
			p.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: blockName.LabelPos.Position(),
				// Add 1 to the end position to add in the end quote, which is stripped from the label value.
				EndPos:  blockName.LabelPos.Add(len(blockName.Label) + 1).Position(),
				Message: `attribute names may not have labels`,
			})
		}

		return &ast.AttributeStmt{
			Name: &ast.Ident{
				Name:    blockName.Fragments[0],
				NamePos: blockName.Start,
			},
			Value: p.ParseExpression(),
		}

	case token.LCURLY: // Block
		block := &ast.BlockStmt{
			Name:     blockName.Fragments,
			NamePos:  blockName.Start,
			Label:    blockName.Label,
			LabelPos: blockName.LabelPos,
		}

		block.LCurlyPos, _, _ = p.expect(token.LCURLY)
		block.Body = p.parseBody(token.RCURLY)
		block.RCurlyPos, _, _ = p.expect(token.RCURLY)

		return block

	default:
		if blockName.ValidAttribute() {
			// The blockname could be used for an attribute or a block (no label,
			// only one name fragment), so inform the user of both cases.
			p.addErrorf("expected attribute assignment or block body, got %s", p.tok)
		} else {
			p.addErrorf("expected block body, got %s", p.tok)
		}

		// Give up on this statement and skip to the next identifier.
		p.advance(token.IDENT)
		return nil
	}
}

// parseBlockName parses the name used for a block.
//
//	BlockName = identifier { "." identifier } [ string ]
func (p *parser) parseBlockName() *blockName {
	if p.tok != token.IDENT {
		p.addErrorf("expected identifier, got %s", p.tok)
		return nil
	}

	var bn blockName

	bn.Fragments = append(bn.Fragments, p.lit) // Append first identifier
	bn.Start = p.pos
	p.next()

	// { "." identifier }
	for p.tok == token.DOT {
		p.next() // consume "."

		if p.tok != token.IDENT {
			p.addErrorf("expected identifier, got %s", p.tok)

			// Continue here to parse as much as possible, even though the block name
			// will be malformed.
		}

		bn.Fragments = append(bn.Fragments, p.lit)
		p.next()
	}

	// [ string ]
	if p.tok != token.ASSIGN && p.tok != token.LCURLY {
		if p.tok == token.STRING {
			// Only allow double-quoted strings for block labels.
			if p.lit[0] != '"' {
				p.addErrorf("expected block label to be a double quoted string, but got %q", p.lit)
			}

			// Strip the quotes if it's non-empty. We then require any non-empty
			// label to be a valid identifier.
			if len(p.lit) > 2 {
				bn.Label = p.lit[1 : len(p.lit)-1]
				if !scanner.IsValidIdentifier(bn.Label) {
					p.addErrorf("expected block label to be a valid identifier, but got %q", bn.Label)
				}
			}
			bn.LabelPos = p.pos
		} else {
			p.addErrorf("expected block label, got %s", p.tok)
		}
		p.next()
	}

	return &bn
}

type blockName struct {
	Fragments []string // Name fragments (i.e., `a.b.c`)
	Label     string   // Optional user label

	Start    token.Pos
	LabelPos token.Pos
}

// ValidAttribute returns true if the blockName can be used as an attribute
// name.
func (n blockName) ValidAttribute() bool {
	return len(n.Fragments) == 1 && n.Label == ""
}

// ParseExpression parses a single expression.
//
//	Expression = BinOpExpr
func (p *parser) ParseExpression() ast.Expr {
	return p.parseBinOp(1)
}

// parseBinOp is the entrypoint for binary expressions. If there is no binary
// expressions in the current state, a single operand will be returned instead.
//
//	BinOpExpr = OrExpr
//	OrExpr    = AndExpr { "||"   AndExpr }
//	AndExpr   = CmpExpr { "&&"   CmpExpr }
//	CmpExpr   = AddExpr { cmp_op AddExpr }
//	AddExpr   = MulExpr { add_op MulExpr }
//	MulExpr   = PowExpr { mul_op PowExpr }
//
// parseBinOp avoids the need for multiple non-terminal functions by providing
// context for operator precedence in recursive calls. inPrec specifies the
// incoming operator precedence. On the first call to parseBinOp, inPrec should
// be 1.
//
// parseBinOp can only handle left-associative operators, so PowExpr is handled
// by parsePowExpr.
func (p *parser) parseBinOp(inPrec int) ast.Expr {
	// The EBNF documented by the function can be generalized into:
	//
	//     CurPrecExpr = NextPrecExpr { cur_prec_ops NextPrecExpr }
	//
	// The code below implements this specific grammar, continually collecting
	// everything at the same precedence level into the LHS of the expression
	// while recursively calling parseBinOp for higher-precedence operations.

	lhs := p.parsePowExpr()

	for {
		tok, pos, prec := p.tok, p.pos, p.tok.BinaryPrecedence()
		if prec < inPrec {
			// The next operator is lower precedence; drop up a level in our call
			// stack.
			return lhs
		}
		p.next() // Consume the operator

		// Recurse with a higher precedence level, which ensures that operators at
		// the same precedence level don't get handled in the recursive call.
		rhs := p.parseBinOp(prec + 1)

		lhs = &ast.BinaryExpr{
			Left:    lhs,
			Kind:    tok,
			KindPos: pos,
			Right:   rhs,
		}
	}
}

// parsePowExpr is like parseBinOp but handles the right-associative pow
// operator.
//
//	PowExpr = UnaryExpr [ "^" PowExpr ]
func (p *parser) parsePowExpr() ast.Expr {
	lhs := p.parseUnaryExpr()

	if p.tok == token.POW {
		pos := p.pos
		p.next() // Consume ^

		return &ast.BinaryExpr{
			Left:    lhs,
			Kind:    token.POW,
			KindPos: pos,
			Right:   p.parsePowExpr(),
		}
	}

	return lhs
}

// parseUnaryExpr parses a unary expression.
//
//	UnaryExpr = OperExpr | unary_op UnaryExpr
//
//	OperExpr   = PrimaryExpr { AccessExpr | IndexExpr | CallExpr }
//	AccessExpr = "." identifier
//	IndexExpr  = "[" Expression "]"
//	CallExpr   = "(" [ ExpressionList ] ")"
func (p *parser) parseUnaryExpr() ast.Expr {
	if isUnaryOp(p.tok) {
		op, pos := p.tok, p.pos
		p.next() // Consume op

		return &ast.UnaryExpr{
			Kind:    op,
			KindPos: pos,
			Value:   p.parseUnaryExpr(),
		}
	}

	primary := p.parsePrimaryExpr()

NextOper:
	for {
		switch p.tok {
		case token.DOT: // AccessExpr
			p.next()
			namePos, _, name := p.expect(token.IDENT)

			primary = &ast.AccessExpr{
				Value: primary,
				Name: &ast.Ident{
					Name:    name,
					NamePos: namePos,
				},
			}

		case token.LBRACK: // IndexExpr
			lBrack, _, _ := p.expect(token.LBRACK)
			index := p.ParseExpression()
			rBrack, _, _ := p.expect(token.RBRACK)

			primary = &ast.IndexExpr{
				Value:     primary,
				LBrackPos: lBrack,
				Index:     index,
				RBrackPos: rBrack,
			}

		case token.LPAREN: // CallExpr
			var args []ast.Expr

			lParen, _, _ := p.expect(token.LPAREN)
			if p.tok != token.RPAREN {
				args = p.parseExpressionList(token.RPAREN)
			}
			rParen, _, _ := p.expect(token.RPAREN)

			primary = &ast.CallExpr{
				Value:     primary,
				LParenPos: lParen,
				Args:      args,
				RParenPos: rParen,
			}

		case token.STRING, token.LCURLY:
			// A user might be trying to assign a block to an attribute. let's
			// attempt to parse the remainder as a block to tell them something is
			// wrong.
			//
			// If we can't parse the remainder of the expression as a block, we give
			// up and parse the remainder of the entire statement.
			if p.tok == token.STRING {
				p.next()
			}
			if _, tok, _ := p.expect(token.LCURLY); tok != token.LCURLY {
				p.consumeStatement()
				return primary
			}
			p.parseBody(token.RCURLY)

			end, tok, _ := p.expect(token.RCURLY)
			if tok != token.RCURLY {
				p.consumeStatement()
				return primary
			}

			p.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(primary).Position(),
				EndPos:   end.Position(),
				Message:  "cannot use a block as an expression",
			})

		default:
			break NextOper
		}
	}

	return primary
}

func isUnaryOp(tok token.Token) bool {
	switch tok {
	case token.NOT, token.SUB:
		return true
	default:
		return false
	}
}

// parsePrimaryExpr parses a primary expression.
//
//	PrimaryExpr = LiteralValue | ArrayExpr | ObjectExpr
//
//	LiteralValue = identifier | string | number | float | bool | null |
//	               "(" Expression ")"
//
//	ArrayExpr  = "[" [ ExpressionList ] "]"
//	ObjectExpr = "{" [ FieldList ] "}"
func (p *parser) parsePrimaryExpr() ast.Expr {
	switch p.tok {
	case token.IDENT:
		res := &ast.IdentifierExpr{
			Ident: &ast.Ident{
				Name:    p.lit,
				NamePos: p.pos,
			},
		}
		p.next()
		return res

	case token.STRING, token.NUMBER, token.FLOAT, token.BOOL, token.NULL:
		res := &ast.LiteralExpr{
			Kind:     p.tok,
			Value:    p.lit,
			ValuePos: p.pos,
		}
		p.next()
		return res

	case token.LPAREN:
		lParen, _, _ := p.expect(token.LPAREN)
		expr := p.ParseExpression()
		rParen, _, _ := p.expect(token.RPAREN)

		return &ast.ParenExpr{
			LParenPos: lParen,
			Inner:     expr,
			RParenPos: rParen,
		}

	case token.LBRACK:
		var res ast.ArrayExpr

		res.LBrackPos, _, _ = p.expect(token.LBRACK)
		if p.tok != token.RBRACK {
			res.Elements = p.parseExpressionList(token.RBRACK)
		}
		res.RBrackPos, _, _ = p.expect(token.RBRACK)
		return &res

	case token.LCURLY:
		var res ast.ObjectExpr

		res.LCurlyPos, _, _ = p.expect(token.LCURLY)
		if p.tok != token.RBRACK {
			res.Fields = p.parseFieldList(token.RCURLY)
		}
		res.RCurlyPos, _, _ = p.expect(token.RCURLY)
		return &res
	}

	p.addErrorf("expected expression, got %s", p.tok)
	res := &ast.LiteralExpr{Kind: token.NULL, Value: "null", ValuePos: p.pos}
	p.advanceAny(statementEnd) // Eat up the rest of the line
	return res
}

var statementEnd = map[token.Token]struct{}{
	token.TERMINATOR: {},
	token.RPAREN:     {},
	token.RCURLY:     {},
	token.RBRACK:     {},
	token.COMMA:      {},
}

// parseExpressionList parses a list of expressions.
//
//	ExpressionList = Expression { "," Expression } [ "," ]
func (p *parser) parseExpressionList(until token.Token) []ast.Expr {
	var exprs []ast.Expr

	for p.tok != until && p.tok != token.EOF {
		exprs = append(exprs, p.ParseExpression())

		if p.tok == until {
			break
		}
		if p.tok != token.COMMA {
			p.addErrorf("missing ',' in expression list")
		}
		p.next()
	}

	return exprs
}

// parseFieldList parses a list of fields in an object.
//
//	FieldList = Field { "," Field } [ "," ]
func (p *parser) parseFieldList(until token.Token) []*ast.ObjectField {
	var fields []*ast.ObjectField

	for p.tok != until && p.tok != token.EOF {
		fields = append(fields, p.parseField())

		if p.tok == until {
			break
		}
		if p.tok != token.COMMA {
			p.addErrorf("missing ',' in field list")
		}
		p.next()
	}

	return fields
}

// parseField parses a field in an object.
//
//	Field = ( string | identifier ) "=" Expression
func (p *parser) parseField() *ast.ObjectField {
	var field ast.ObjectField

	if p.tok == token.STRING || p.tok == token.IDENT {
		field.Name = &ast.Ident{
			Name:    p.lit,
			NamePos: p.pos,
		}
		if p.tok == token.STRING && len(p.lit) > 2 {
			// The field name is a string literal; unwrap the quotes.
			field.Name.Name = p.lit[1 : len(p.lit)-1]
			field.Quoted = true
		}
		p.next() // Consume field name
	} else {
		p.addErrorf("expected field name (string or identifier), got %s", p.tok)
		p.advance(token.ASSIGN)
	}

	p.expect(token.ASSIGN)

	field.Value = p.ParseExpression()
	return &field
}
