package builder

import (
	"bytes"
	"io"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/printer"
	"github.com/grafana/river/token"
)

// A Token is a wrapper around token.Token which contains the token type
// alongside its literal. Use LiteralTok as the Tok field to write literal
// characters such as whitespace.
type Token struct {
	Tok token.Token
	Lit string
}

// printFileTokens prints out the tokens as River text and formats them, writing
// the final result to w.
func printFileTokens(w io.Writer, toks []Token) (int, error) {
	var raw bytes.Buffer
	for _, tok := range toks {
		switch {
		case tok.Tok == token.LITERAL:
			raw.WriteString(tok.Lit)
		case tok.Tok == token.COMMENT:
			raw.WriteString(tok.Lit)
		case tok.Tok.IsLiteral() || tok.Tok.IsKeyword():
			raw.WriteString(tok.Lit)
		default:
			raw.WriteString(tok.Tok.String())
		}
	}

	f, err := parser.ParseFile("", raw.Bytes())
	if err != nil {
		return 0, err
	}

	wc := &writerCount{w: w}
	err = printer.Fprint(wc, f)
	return wc.n, err
}

// printExprTokens prints out the tokens as River text and formats them,
// writing the final result to w.
func printExprTokens(w io.Writer, toks []Token) (int, error) {
	var raw bytes.Buffer
	for _, tok := range toks {
		switch {
		case tok.Tok == token.LITERAL:
			raw.WriteString(tok.Lit)
		case tok.Tok.IsLiteral() || tok.Tok.IsKeyword():
			raw.WriteString(tok.Lit)
		default:
			raw.WriteString(tok.Tok.String())
		}
	}

	expr, err := parser.ParseExpression(raw.String())
	if err != nil {
		return 0, err
	}

	wc := &writerCount{w: w}
	err = printer.Fprint(wc, expr)
	return wc.n, err
}

type writerCount struct {
	w io.Writer
	n int
}

func (wc *writerCount) Write(p []byte) (n int, err error) {
	n, err = wc.w.Write(p)
	wc.n += n
	return
}
