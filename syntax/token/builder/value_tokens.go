package builder

import (
	"fmt"
	"sort"

	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/scanner"
	"github.com/grafana/river/token"
)

// TODO(rfratto): check for optional values

// Tokenizer is any value which can return a raw set of tokens.
type Tokenizer interface {
	// RiverTokenize returns the raw set of River tokens which are used when
	// printing out the value with river/token/builder.
	RiverTokenize() []Token
}

func tokenEncode(val interface{}) []Token {
	return valueTokens(value.Encode(val))
}

func valueTokens(v value.Value) []Token {
	var toks []Token

	// If v is a Tokenizer, allow it to override what tokens get generated.
	if tk, ok := v.Interface().(Tokenizer); ok {
		return tk.RiverTokenize()
	}

	switch v.Type() {
	case value.TypeNull:
		toks = append(toks, Token{token.NULL, "null"})

	case value.TypeNumber:
		toks = append(toks, Token{token.NUMBER, v.Number().ToString()})

	case value.TypeString:
		toks = append(toks, Token{token.STRING, fmt.Sprintf("%q", v.Text())})

	case value.TypeBool:
		toks = append(toks, Token{token.STRING, fmt.Sprintf("%v", v.Bool())})

	case value.TypeArray:
		toks = append(toks, Token{token.LBRACK, ""})
		elems := v.Len()
		for i := 0; i < elems; i++ {
			elem := v.Index(i)

			toks = append(toks, valueTokens(elem)...)
			if i+1 < elems {
				toks = append(toks, Token{token.COMMA, ""})
			}
		}
		toks = append(toks, Token{token.RBRACK, ""})

	case value.TypeObject:
		toks = append(toks, Token{token.LCURLY, ""}, Token{token.LITERAL, "\n"})

		keys := v.Keys()

		// If v isn't an ordered object (i.e., a go map), sort the keys so they
		// have a deterministic print order.
		if !v.OrderedKeys() {
			sort.Strings(keys)
		}

		for i := 0; i < len(keys); i++ {
			if scanner.IsValidIdentifier(keys[i]) {
				toks = append(toks, Token{token.IDENT, keys[i]})
			} else {
				toks = append(toks, Token{token.STRING, fmt.Sprintf("%q", keys[i])})
			}

			field, _ := v.Key(keys[i])
			toks = append(toks, Token{token.ASSIGN, ""})
			toks = append(toks, valueTokens(field)...)
			toks = append(toks, Token{token.COMMA, ""}, Token{token.LITERAL, "\n"})
		}
		toks = append(toks, Token{token.RCURLY, ""})

	case value.TypeFunction:
		toks = append(toks, Token{token.LITERAL, v.Describe()})

	case value.TypeCapsule:
		toks = append(toks, Token{token.LITERAL, v.Describe()})

	default:
		panic(fmt.Sprintf("river/token/builder: unrecognized value type %q", v.Type()))
	}

	return toks
}
