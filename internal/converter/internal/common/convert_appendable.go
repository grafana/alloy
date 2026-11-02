package common

import (
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/prometheus/prometheus/storage"
)

// ConvertAppendable implements both the [builder.Tokenizer] and
// [storage.AppendableV2] interfaces. This allows us to set component.Arguments
// that leverage [storage.AppendableV2] with an implementation that can be
// tokenized as a specific string.
type ConvertAppendable struct {
	storage.AppendableV2

	Expr string // The specific string to return during tokenization.
}

var _ storage.AppendableV2 = (*ConvertAppendable)(nil)
var _ builder.Tokenizer = ConvertAppendable{}
var _ syntax.Capsule = ConvertAppendable{}

func (f ConvertAppendable) AlloyCapsule() {}
func (f ConvertAppendable) AlloyTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: f.Expr,
	}}
}
