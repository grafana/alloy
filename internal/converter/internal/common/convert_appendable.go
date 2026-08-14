package common

import (
	"context"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/prometheus/prometheus/storage"
)

// ConvertAppendable implements both the [builder.Tokenizer] and
// [storage.Appendable]/[storage.AppendableV2] interfaces. This allows us to
// set component.Arguments that leverage [storage.AppendableV2] with an
// implementation that can be tokenized as a specific string.
type ConvertAppendable struct {
	storage.Appendable

	Expr string // The specific string to return during tokenization.
}

var _ storage.Appendable = (*ConvertAppendable)(nil)
var _ storage.AppendableV2 = (*ConvertAppendable)(nil)
var _ builder.Tokenizer = ConvertAppendable{}
var _ syntax.Capsule = ConvertAppendable{}

// AppenderV2 implements storage.AppendableV2. ConvertAppendable is only used
// during config conversion for tokenization; AppenderV2 is never called.
func (f ConvertAppendable) AppenderV2(_ context.Context) storage.AppenderV2 {
	panic("ConvertAppendable.AppenderV2 must not be called; it is only used for config conversion tokenization")
}

func (f ConvertAppendable) AlloyCapsule() {}
func (f ConvertAppendable) AlloyTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: f.Expr,
	}}
}
