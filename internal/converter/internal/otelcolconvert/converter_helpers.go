package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/mitchellh/mapstructure"
)

// This file contains shared helpers for converters to use.

// tokenizedConsumer implements [otelcol.Consumer] and [builder.Tokenizer].
// tokenizedConsumer tokenizes as the string literal specified by the Expr
// field.
type tokenizedConsumer struct {
	otelcol.Consumer

	Expr string // Expr is the string to return during tokenization.
}

func (tc tokenizedConsumer) AlloyCapsule() {}

func (tc tokenizedConsumer) AlloyTokenize() []builder.Token {
	return []builder.Token{{
		Tok: token.STRING,
		Lit: tc.Expr,
	}}
}

func ToTokenizedConsumers(components []componentID) []otelcol.Consumer {
	if len(components) == 0 {
		return nil
	}

	res := make([]otelcol.Consumer, 0, len(components))

	for _, component := range components {
		res = append(res, tokenizedConsumer{
			Expr: fmt.Sprintf("%s.%s.input", strings.Join(component.Name, "."), component.Label),
		})
	}

	return res
}

// encodeMapstruct uses mapstruct fields to convert the given argument into a
// map[string]any. This is useful for being able to convert configuration
// sections for OpenTelemetry components where the configuration type is hidden
// in an internal package.
func encodeMapstruct(v any) map[string]any {
	var res map[string]any
	var decoderConfig mapstructure.DecoderConfig = mapstructure.DecoderConfig{Squash: true, Result: &res}
	decoder, err := mapstructure.NewDecoder(&decoderConfig)
	if err != nil {
		panic(err)
	}

	err = decoder.Decode(v)
	if err != nil {
		panic(err)
	}
	return res
}

// encodeMapslice uses mapstruct fields to convert the given argument into a
// []map[string]any. This is useful for being able to convert configuration
// sections for OpenTelemetry components where the configuration type is hidden
// in an internal package.
func encodeMapslice(v any) []map[string]any {
	var res []map[string]any
	if err := mapstructure.Decode(v, &res); err != nil {
		panic(err)
	}
	return res
}

// encodeString uses mapstruct fields to convert the given argument into a
// string. This is useful for being able to convert configuration
// sections for OpenTelemetry components where the configuration type is hidden
// in an internal package.
func encodeString(v any) string {
	var res string
	if err := mapstructure.Decode(v, &res); err != nil {
		panic(err)
	}
	return res
}
