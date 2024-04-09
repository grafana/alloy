package common

import (
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
)

type CustomTokenizer struct {
	Expr string
}

var _ builder.Tokenizer = CustomTokenizer{}

func (f CustomTokenizer) AlloyTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: f.Expr,
	}}
}
