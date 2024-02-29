package common

import (
	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/river"
	"github.com/grafana/river/token"
	"github.com/grafana/river/token/builder"
)

// ConvertLogsReceiver allows us to override how the loki.LogsReceiver is tokenized.
// See ConvertAppendable as another example with more details in comments.
type ConvertLogsReceiver struct {
	loki.LogsReceiver

	Expr string
}

var _ loki.LogsReceiver = (*ConvertLogsReceiver)(nil)
var _ builder.Tokenizer = ConvertLogsReceiver{}
var _ river.Capsule = ConvertLogsReceiver{}

func (f ConvertLogsReceiver) RiverCapsule() {}
func (f ConvertLogsReceiver) RiverTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: f.Expr,
	}}
}
