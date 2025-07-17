package repl

import (
	"github.com/c-bata/go-prompt"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/graphql"
	"github.com/spf13/cobra"
)

type AlloyRepl struct {
	HttpAddr             string
	MinStability         featuregate.Stability
	EnableCommunityComps bool
}

func (fr *AlloyRepl) Run(cmd *cobra.Command) error {
	client := graphql.NewGraphQlClient(fr.HttpAddr)

	p := prompt.New(
		NewExecutor(fr, client).Execute,
		NewCompleter(fr, client).Complete,
		prompt.OptionTitle("alloy-repl: interactive alloy diagnostics"),
		prompt.OptionPrefix("alloy >> "),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{'('},
			Fn:        insertCharPair("(  )"),
		}),
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{'{'},
			Fn:        insertCharPair("{  }"),
		}),
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{'"'},
			Fn:        insertCharPair("\"\""),
		}),
	)
	p.Run()

	return nil
}

func insertCharPair(pair string) func(buf *prompt.Buffer) {
	return func(buf *prompt.Buffer) {
		if buf.Document().CurrentLineAfterCursor() == "" || buf.Document().CurrentLineAfterCursor()[:1] == " " {
			buf.InsertText(pair, false, false)
			buf.CursorRight(len(pair) / 2)
		} else {
			buf.InsertText(pair[:1], false, false)
			buf.CursorRight(1)
		}
	}
}
