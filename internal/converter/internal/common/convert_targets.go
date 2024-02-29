package common

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/river"
	"github.com/grafana/river/token"
	"github.com/grafana/river/token/builder"
)

// NewDiscoveryExports will return a new [discovery.Exports] with a specific
// key for converter component exports. The argument will be tokenized
// as a component export string rather than the standard [discovery.Target]
// RiverTokenize.
func NewDiscoveryExports(expr string) discovery.Exports {
	return discovery.Exports{
		Targets: NewDiscoveryTargets(expr),
	}
}

// NewDiscoveryTargets will return a new [[]discovery.Target] with a specific
// key for converter component exports. The argument will be tokenized
// as a component export string rather than the standard [discovery.Target]
// RiverTokenize.
func NewDiscoveryTargets(expr string) []discovery.Target {
	return []discovery.Target{map[string]string{"__expr__": expr}}
}

// ConvertTargets implements [builder.Tokenizer]. This allows us to set
// component.Arguments with an implementation that can be tokenized with
// custom behaviour for converting.
type ConvertTargets struct {
	Targets []discovery.Target
}

var _ builder.Tokenizer = ConvertTargets{}
var _ river.Capsule = ConvertTargets{}

func (f ConvertTargets) RiverCapsule() {}
func (f ConvertTargets) RiverTokenize() []builder.Token {
	expr := builder.NewExpr()
	var toks []builder.Token

	targetCount := len(f.Targets)
	if targetCount == 0 {
		expr.SetValue(f.Targets)
		return expr.Tokens()
	}

	if targetCount > 1 {
		toks = append(toks, builder.Token{Tok: token.LITERAL, Lit: "concat"})
		toks = append(toks, builder.Token{Tok: token.LPAREN})
		toks = append(toks, builder.Token{Tok: token.LITERAL, Lit: "\n"})
	}

	for ix, targetMap := range f.Targets {
		keyValMap := map[string]string{}
		for key, val := range targetMap {
			// __expr__ is a special key used by the converter code to specify
			// we should tokenize the value instead of tokenizing the map normally.
			// An alternative strategy would have been to add a new property for
			// token override to the upstream type discovery.Target.
			if key == "__expr__" {
				toks = append(toks, builder.Token{Tok: token.LITERAL, Lit: val})
				if ix != len(f.Targets)-1 {
					toks = append(toks, builder.Token{Tok: token.COMMA})
					toks = append(toks, builder.Token{Tok: token.LITERAL, Lit: "\n"})
				}
			} else {
				keyValMap[key] = val
			}
		}

		if len(keyValMap) > 0 {
			expr.SetValue([]map[string]string{keyValMap})
			toks = append(toks, expr.Tokens()...)
			if ix != len(f.Targets)-1 {
				toks = append(toks, builder.Token{Tok: token.COMMA})
				toks = append(toks, builder.Token{Tok: token.LITERAL, Lit: "\n"})
			}
		}
	}

	if targetCount > 1 {
		toks = append(toks, builder.Token{Tok: token.COMMA})
		toks = append(toks, builder.Token{Tok: token.LITERAL, Lit: "\n"})
		toks = append(toks, builder.Token{Tok: token.RPAREN})
	}

	return toks
}
