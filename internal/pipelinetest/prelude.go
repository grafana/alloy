package pipelinetest

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/syntax/token"
	builder "github.com/grafana/alloy/syntax/token/builder"
)

func withPrelude(schema TestSchema) string {
	sink := buildSink()
	sources := buildSources(schema.Inputs)

	return sink + "\n\n" + sources + "\n\n" + rewritePipelineTestRefs(schema.Config)
}

func rewritePipelineTestRefs(source string) string {
	replacer := strings.NewReplacer(
		"pipelinetest.loki.url", "pipelinetest.sink.out.loki_push_url",
		"pipelinetest.loki.receiver", "pipelinetest.sink.out.lokireceiver",
	)
	return replacer.Replace(source)
}

func buildSink() string {
	return `pipelinetest.sink "out" {}`
}

func buildSources(inputs InputSchema) string {
	file := builder.NewFile()

	for i, input := range inputs.Loki {
		if len(input.Components) == 0 {
			continue
		}

		source := builder.NewBlock([]string{"pipelinetest", "source"}, generatedLokiInputComponentName(i))
		forwardTo := builder.NewBlock([]string{"forward_to"}, "")
		forwardTo.Body().SetAttributeTokens("logs", receiverListTokens(input.Components))
		source.Body().AppendBlock(forwardTo)
		file.Body().AppendBlock(source)
	}

	if len(file.Body().Nodes()) == 0 {
		return ""
	}

	return string(file.Bytes())
}

func generatedLokiInputComponentID(i int) string {
	return "pipelinetest.source." + generatedLokiInputComponentName(i)
}

func generatedLokiInputComponentName(i int) string {
	return fmt.Sprintf("__pipelinetest_loki_input_%d", i)
}

func receiverListTokens(receivers []string) []builder.Token {
	tokens := make([]builder.Token, 0, len(receivers)*4+2)
	tokens = append(tokens, builder.Token{Tok: token.LBRACK})

	for i, receiver := range receivers {
		if i > 0 {
			tokens = append(tokens, builder.Token{Tok: token.COMMA})
		}

		for j, frag := range splitReceiver(receiver) {
			if j > 0 {
				tokens = append(tokens, builder.Token{Tok: token.DOT})
			}
			tokens = append(tokens, builder.Token{Tok: token.IDENT, Lit: frag})
		}
	}

	tokens = append(tokens, builder.Token{Tok: token.RBRACK})
	return tokens
}

func splitReceiver(receiver string) []string {
	var out []string
	start := 0
	for i := 0; i < len(receiver); i++ {
		if receiver[i] != '.' {
			continue
		}
		out = append(out, receiver[start:i])
		start = i + 1
	}
	out = append(out, receiver[start:])
	return out
}
