// Package printer contains utilities for pretty-printing River ASTs.
package printer

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"

	"github.com/grafana/river/ast"
	"github.com/grafana/river/token"
)

// Config configures behavior of the printer.
type Config struct {
	Indent int // Indentation to apply to all emitted code. Default 0.
}

// Fprint pretty-prints the specified node to w. The Node type must be an
// *ast.File, ast.Body, or a type that implements ast.Stmt or ast.Expr.
func (c *Config) Fprint(w io.Writer, node ast.Node) (err error) {
	var p printer
	p.Init(c)

	// Pass all of our text through a trimmer to ignore trailing whitespace.
	w = &trimmer{next: w}

	if err = (&walker{p: &p}).Walk(node); err != nil {
		return
	}

	// Call flush one more time to write trailing comments.
	p.flush(token.Position{
		Offset: math.MaxInt,
		Line:   math.MaxInt,
		Column: math.MaxInt,
	}, token.EOF)

	w = tabwriter.NewWriter(w, 0, 8, 1, ' ', tabwriter.DiscardEmptyColumns|tabwriter.TabIndent)

	if _, err = w.Write(p.output); err != nil {
		return
	}
	if tw, _ := w.(*tabwriter.Writer); tw != nil {
		// Flush tabwriter if defined
		err = tw.Flush()
	}

	return
}

// Fprint pretty-prints the specified node to w. The Node type must be an
// *ast.File, ast.Body, or a type that implements ast.Stmt or ast.Expr.
func Fprint(w io.Writer, node ast.Node) error {
	c := &Config{}
	return c.Fprint(w, node)
}

// The printer writes lexical tokens and whitespace to an internal buffer.
// Comments are written by the printer itself, while all other tokens and
// formatting characters are sent through calls to Write.
//
// Internally, printer depends on a tabwriter for formatting text and aligning
// runs of characters. Horizontal '\t' and vertical '\v' tab characters are
// used to introduce new columns in the row. Runs of characters are stopped
// be either introducing a linefeed '\f' or by having a line with a different
// number of columns from the previous line. See the text/tabwriter package for
// more information on the elastic tabstop algorithm it uses for formatting
// text.
type printer struct {
	cfg Config

	// State variables

	output  []byte
	indent  int         // Current indentation level
	lastTok token.Token // Last token printed (token.LITERAL if it's whitespace)

	// Whitespace holds a buffer of whitespace characters to print prior to the
	// next non-whitespace token. Whitespace is held in a buffer to avoid
	// printing unnecessary whitespace at the end of a file.
	whitespace []whitespace

	// comments stores comments to be processed as elements are printed.
	comments commentInfo

	// pos is an approximation of the current position in AST space, and is used
	// to determine space between AST elements (e.g., if a comment should come
	// before a token). pos automatically as elements are written and can be manually
	// set to guarantee an accurate position by passing a token.Pos to Write.
	pos  token.Position
	last token.Position // Last pos written to output (through writeString)

	// out is an accurate representation of the current position in output space,
	// used to inject extra formatting like indentation based on the output
	// position.
	//
	// out may differ from pos in terms of whitespace.
	out token.Position
}

type commentInfo struct {
	list []ast.CommentGroup
	idx  int
	cur  ast.CommentGroup
	pos  token.Pos
}

func (ci *commentInfo) commentBefore(next token.Position) bool {
	return ci.pos != token.NoPos && ci.pos.Offset() <= next.Offset
}

// nextComment preloads the next comment.
func (ci *commentInfo) nextComment() {
	for ci.idx < len(ci.list) {
		c := ci.list[ci.idx]
		ci.idx++
		if len(c) > 0 {
			ci.cur = c
			ci.pos = ast.StartPos(c[0])
			return
		}
	}
	ci.pos = token.NoPos
}

// Init initializes the printer for printing. Init is intended to be called
// once per printer and doesn't fully reset its state.
func (p *printer) Init(cfg *Config) {
	p.cfg = *cfg
	p.pos = token.Position{Line: 1, Column: 1}
	p.out = token.Position{Line: 1, Column: 1}
	// Capacity is set low since most whitespace sequences are short.
	p.whitespace = make([]whitespace, 0, 16)
}

// SetComments set the comments to use.
func (p *printer) SetComments(comments []ast.CommentGroup) {
	p.comments = commentInfo{
		list: comments,
		idx:  0,
		pos:  token.NoPos,
	}
	p.comments.nextComment()
}

// Write writes a list of writable arguments to the printer.
//
// Arguments can be one of the types described below:
//
// If arg is a whitespace value, it is accumulated into a buffer and flushed
// only after a non-whitespace value is processed. The whitespace buffer will
// be forcibly flushed if the buffer becomes full without writing a
// non-whitespace token.
//
// If arg is an *ast.IdentifierExpr, *ast.LiteralExpr, or a token.Token, the
// human-readable representation of that value will be written.
//
// When writing text, comments which need to appear before that text in
// AST-space are written first, followed by leftover whitespace and then the
// text to write. The written text will update the AST-space position.
//
// If arg is a token.Pos, the AST-space position of the printer is updated to
// the provided Pos. Writing token.Pos values can help make sure the printer's
// AST-space position is accurate, as AST-space position is otherwise an
// estimation based on written data.
func (p *printer) Write(args ...interface{}) {
	for _, arg := range args {
		var (
			data  string
			isLit bool
		)

		switch arg := arg.(type) {
		case whitespace:
			// Whitespace token; add it to our token buffer. Note that a whitespace
			// token is different than the actual whitespace which will get written
			// (e.g., wsIndent increases indentation level by one instead of setting
			// it to one.)
			if arg == wsIgnore {
				continue
			}
			i := len(p.whitespace)
			if i == cap(p.whitespace) {
				// We built up too much whitespace; this can happen if too many calls
				// to Write happen without appending a non-comment token. We will
				// force-flush the existing whitespace to avoid a panic.
				//
				// Ideally this line is never hit based on how we walk the AST, but
				// it's kept for safety.
				p.writeWritespace(i)
				i = 0
			}
			p.whitespace = p.whitespace[0 : i+1]
			p.whitespace[i] = arg
			p.lastTok = token.LITERAL
			continue

		case *ast.Ident:
			data = arg.Name
			p.lastTok = token.IDENT

		case *ast.LiteralExpr:
			data = arg.Value
			p.lastTok = arg.Kind

		case token.Pos:
			if arg.Valid() {
				p.pos = arg.Position()
			}
			// Don't write anything; token.Pos is an instruction and doesn't include
			// any text to write.
			continue

		case token.Token:
			s := arg.String()
			data = s

			// We will need to inject whitespace if the previous token and the
			// current token would combine into a single token when re-scanned. This
			// ensures that the sequence of tokens emitted by the output of the
			// printer match the sequence of tokens from the input.
			if mayCombine(p.lastTok, s[0]) {
				if len(p.whitespace) != 0 {
					// It shouldn't be possible for the whitespace buffer to be not empty
					// here; p.lastTok would've had to been a non-whitespace token and so
					// whitespace would've been flushed when it was written to the output
					// buffer.
					panic("whitespace buffer not empty")
				}
				p.whitespace = p.whitespace[0:1]
				p.whitespace[0] = ' '
			}
			p.lastTok = arg

		default:
			panic(fmt.Sprintf("printer: unsupported argument %v (%T)\n", arg, arg))
		}

		next := p.pos

		p.flush(next, p.lastTok)
		p.writeString(next, data, isLit)
	}
}

// mayCombine returns true if two tokes must not be combined, because combining
// them would format in a different token sequence being generated.
func mayCombine(prev token.Token, next byte) (b bool) {
	switch prev {
	case token.NUMBER:
		return next == '.' // 1.
	case token.DIV:
		return next == '*' // /*
	default:
		return false
	}
}

// flush prints any pending comments and whitespace occurring textually before
// the position of the next token tok. The flush result indicates if a newline
// was written or if a formfeed \f character was dropped from the whitespace
// buffer.
func (p *printer) flush(next token.Position, tok token.Token) {
	if p.comments.commentBefore(next) {
		p.injectComments(next, tok)
	} else if tok != token.EOF {
		// Write all remaining whitespace.
		p.writeWritespace(len(p.whitespace))
	}
}

func (p *printer) injectComments(next token.Position, tok token.Token) {
	var lastComment *ast.Comment

	for p.comments.commentBefore(next) {
		for _, c := range p.comments.cur {
			p.writeCommentPrefix(next, c)
			p.writeComment(next, c)
			lastComment = c
		}
		p.comments.nextComment()
	}

	p.writeCommentSuffix(next, tok, lastComment)
}

// writeCommentPrefix writes whitespace that should appear before c.
func (p *printer) writeCommentPrefix(next token.Position, c *ast.Comment) {
	if len(p.output) == 0 {
		// The comment is the first thing written to the output. Don't write any
		// whitespace before it.
		return
	}

	cPos := c.StartPos.Position()

	if cPos.Line == p.last.Line {
		// Our comment is on the same line as the last token. Write a separator
		// between the last token and the comment.
		separator := byte('\t')
		if cPos.Line == next.Line {
			// The comment is on the same line as the next token, which means it has
			// to be a block comment (since line comments run to the end of the
			// line.) Use a space as the separator instead since a tab in the middle
			// of a line between comments would look weird.
			separator = byte(' ')
		}
		p.writeByte(separator, 1)
	} else {
		// Our comment is on a different line from the last token. First write
		// pending whitespace from the last token up to the first newline.
		var wsCount int

		for i, ws := range p.whitespace {
			switch ws {
			case wsBlank, wsVTab:
				// Drop any whitespace before the comment.
				p.whitespace[i] = wsIgnore
			case wsIndent, wsUnindent:
				// Allow indentation to be applied.
				continue
			case wsNewline, wsFormfeed:
				// Drop the whitespace since we're about to write our own.
				p.whitespace[i] = wsIgnore
			}
			wsCount = i
			break
		}
		p.writeWritespace(wsCount)

		var newlines int
		if cPos.Valid() && p.last.Valid() {
			newlines = cPos.Line - p.last.Line
		}
		if newlines > 0 {
			p.writeByte('\f', newlineLimit(newlines))
		}
	}
}

func (p *printer) writeComment(_ token.Position, c *ast.Comment) {
	p.writeString(c.StartPos.Position(), c.Text, true)
}

// writeCommentSuffix writes any whitespace necessary between the last comment
// and next. lastComment should be the final comment written.
func (p *printer) writeCommentSuffix(next token.Position, tok token.Token, lastComment *ast.Comment) {
	if tok == token.EOF {
		// We don't want to add any blank newlines before the end of the file;
		// return early.
		return
	}

	var droppedFF bool

	// If our final comment is a block comment and is on the same line as the
	// next token, add a space as a suffix to separate them.
	lastCommentPos := ast.EndPos(lastComment).Position()
	if lastComment.Text[1] == '*' && next.Line == lastCommentPos.Line {
		p.writeByte(' ', 1)
	}

	newlines := next.Line - p.last.Line

	for i, ws := range p.whitespace {
		switch ws {
		case wsBlank, wsVTab:
			p.whitespace[i] = wsIgnore
		case wsIndent, wsUnindent:
			continue
		case wsNewline, wsFormfeed:
			if ws == wsFormfeed {
				droppedFF = true
			}
			p.whitespace[i] = wsIgnore
		}
	}

	p.writeWritespace(len(p.whitespace))

	// Write newlines as long as the next token isn't EOF (so that there's no
	// blank newlines at the end of the file).
	if newlines > 0 {
		ch := byte('\n')
		if droppedFF {
			// If we dropped a formfeed while writing comments, we should emit a new
			// one.
			ch = byte('\f')
		}
		p.writeByte(ch, newlineLimit(newlines))
	}
}

// writeString writes the literal string s into the printer's output.
// Formatting characters in s such as '\t' and '\n' will be interpreted by
// underlying tabwriter unless isLit is set.
func (p *printer) writeString(pos token.Position, s string, isLit bool) {
	if p.out.Column == 1 {
		// We haven't written any text to this line yet; prepend our indentation
		// for the line.
		p.writeIndent()
	}

	if pos.Valid() {
		// Update p.pos if pos is valid. This is done *after* handling indentation
		// since we want to interpret pos as the literal position for s (and
		// writeIndent will update p.pos).
		p.pos = pos
	}

	if isLit {
		// Wrap our literal string in tabwriter.Escape if it's meant to be written
		// without interpretation by the tabwriter.
		p.output = append(p.output, tabwriter.Escape)

		defer func() {
			p.output = append(p.output, tabwriter.Escape)
		}()
	}

	p.output = append(p.output, s...)

	var (
		newlines       int
		lastNewlineIdx int
	)

	for i := 0; i < len(s); i++ {
		if ch := s[i]; ch == '\n' || ch == '\f' {
			newlines++
			lastNewlineIdx = i
		}
	}

	p.pos.Offset += len(s)

	if newlines > 0 {
		p.pos.Line += newlines
		p.out.Line += newlines

		newColumn := len(s) - lastNewlineIdx
		p.pos.Column = newColumn
		p.out.Column = newColumn
	} else {
		p.pos.Column += len(s)
		p.out.Column += len(s)
	}

	p.last = p.pos
}

func (p *printer) writeIndent() {
	depth := p.cfg.Indent + p.indent
	for i := 0; i < depth; i++ {
		p.output = append(p.output, '\t')
	}

	p.pos.Offset += depth
	p.pos.Column += depth
	p.out.Column += depth
}

// writeByte writes ch n times to the output, updating the position of the
// printer. writeByte is only used for writing whitespace characters.
func (p *printer) writeByte(ch byte, n int) {
	if p.out.Column == 1 {
		p.writeIndent()
	}

	for i := 0; i < n; i++ {
		p.output = append(p.output, ch)
	}

	// Update positions.
	p.pos.Offset += n
	if ch == '\n' || ch == '\f' {
		p.pos.Line += n
		p.out.Line += n
		p.pos.Column = 1
		p.out.Column = 1
		return
	}
	p.pos.Column += n
	p.out.Column += n
}

// writeWhitespace writes the first n whitespace entries in the whitespace
// buffer.
//
// writeWritespace is only safe to be called when len(p.whitespace) >= n.
func (p *printer) writeWritespace(n int) {
	for i := 0; i < n; i++ {
		switch ch := p.whitespace[i]; ch {
		case wsIgnore: // no-op
		case wsIndent:
			p.indent++
		case wsUnindent:
			p.indent--
			if p.indent < 0 {
				panic("printer: negative indentation")
			}
		default:
			p.writeByte(byte(ch), 1)
		}
	}

	// Shift remaining entries down
	l := copy(p.whitespace, p.whitespace[n:])
	p.whitespace = p.whitespace[:l]
}

const maxNewlines = 2

// newlineLimit limits a newline count to maxNewlines.
func newlineLimit(count int) int {
	if count > maxNewlines {
		count = maxNewlines
	}
	return count
}

// whitespace represents a whitespace token to write to the printer's internal
// buffer.
type whitespace byte

const (
	wsIgnore   = whitespace(0)
	wsBlank    = whitespace(' ')
	wsVTab     = whitespace('\v')
	wsNewline  = whitespace('\n')
	wsFormfeed = whitespace('\f')
	wsIndent   = whitespace('>')
	wsUnindent = whitespace('<')
)

func (ws whitespace) String() string {
	switch ws {
	case wsIgnore:
		return "wsIgnore"
	case wsBlank:
		return "wsBlank"
	case wsVTab:
		return "wsVTab"
	case wsNewline:
		return "wsNewline"
	case wsFormfeed:
		return "wsFormfeed"
	case wsIndent:
		return "wsIndent"
	case wsUnindent:
		return "wsUnindent"
	default:
		return fmt.Sprintf("whitespace(%d)", ws)
	}
}
