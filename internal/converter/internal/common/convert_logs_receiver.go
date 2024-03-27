package common

import (
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
)

// ConvertLogsReceiver allows us to override how the loki.LogsReceiver is tokenized.
// See ConvertAppendable as another example with more details in comments.
type ConvertLogsReceiver struct {
	loki.LogsReceiver

	Expr string
}

var _ loki.LogsReceiver = (*ConvertLogsReceiver)(nil)
var _ builder.Tokenizer = ConvertLogsReceiver{}
var _ syntax.Capsule = ConvertLogsReceiver{}

func (f ConvertLogsReceiver) AlloyCapsule() {}
func (f ConvertLogsReceiver) AlloyTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: f.Expr,
	}}
}
