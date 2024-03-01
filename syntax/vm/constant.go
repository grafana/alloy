package vm

import (
	"fmt"
	"strconv"

	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/token"
)

func valueFromLiteral(lit string, tok token.Token) (value.Value, error) {
	// NOTE(rfratto): this function should never return an error, since the
	// parser only produces valid tokens; it can only fail if a user hand-builds
	// an AST with invalid literals.

	switch tok {
	case token.NULL:
		return value.Null, nil

	case token.NUMBER:
		intVal, err1 := strconv.ParseInt(lit, 0, 64)
		if err1 == nil {
			return value.Int(intVal), nil
		}

		uintVal, err2 := strconv.ParseUint(lit, 0, 64)
		if err2 == nil {
			return value.Uint(uintVal), nil
		}

		floatVal, err3 := strconv.ParseFloat(lit, 64)
		if err3 == nil {
			return value.Float(floatVal), nil
		}

		return value.Null, err3

	case token.FLOAT:
		v, err := strconv.ParseFloat(lit, 64)
		if err != nil {
			return value.Null, err
		}
		return value.Float(v), nil

	case token.STRING:
		v, err := strconv.Unquote(lit)
		if err != nil {
			return value.Null, err
		}
		return value.String(v), nil

	case token.BOOL:
		switch lit {
		case "true":
			return value.Bool(true), nil
		case "false":
			return value.Bool(false), nil
		default:
			return value.Null, fmt.Errorf("invalid boolean literal %q", lit)
		}
	default:
		panic(fmt.Sprintf("%v is not a valid token", tok))
	}
}
