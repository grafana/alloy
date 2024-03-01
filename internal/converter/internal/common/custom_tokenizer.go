package common

import (
	"github.com/grafana/river/token"
	"github.com/grafana/river/token/builder"
)

type CustomTokenizer struct {
	Expr string
}

var _ builder.Tokenizer = CustomTokenizer{}

func (f CustomTokenizer) RiverTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: f.Expr,
	}}
}
