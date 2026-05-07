package common

import (
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
)

// ConvertLogsConsumer allows us to override how the loki.Consumer is tokenized.
// See ConvertAppendable as another example with more details in comments.
type ConvertLogsConsumer struct {
	loki.Consumer

	Expr string
}

var _ loki.Consumer = (*ConvertLogsConsumer)(nil)
var _ builder.Tokenizer = ConvertLogsConsumer{}
var _ syntax.Capsule = ConvertLogsConsumer{}

func (f ConvertLogsConsumer) AlloyCapsule() {}
func (f ConvertLogsConsumer) AlloyTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: f.Expr,
	}}
}
