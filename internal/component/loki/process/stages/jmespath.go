package stages

import (
	"encoding"
	"errors"
	"fmt"

	"github.com/jmespath-community/go-jmespath"
)

func compileJMESPathMap(m map[string]JMESPath) (map[string]jmespath.JMESPath, error) {
	var expressions map[string]jmespath.JMESPath
	for k, expr := range m {
		if expressions == nil {
			expressions = make(map[string]jmespath.JMESPath, 1)
		}

		// If there is no expression, use the name as the expression.
		if expr == "" {
			expr = JMESPath(k)
		}

		var err error
		expressions[k], err = expr.expr()
		if err != nil {
			return nil, err
		}
	}

	return expressions, nil
}

var (
	_ encoding.TextMarshaler   = JMESPath("")
	_ encoding.TextUnmarshaler = (*JMESPath)(nil)
)

var (
	errCouldNotCompileJMES = errors.New("could not compile JMES expression")
)

type JMESPath string

// UnmarshalText implements encoding.TextUnmarshaler.
func (j *JMESPath) UnmarshalText(text []byte) error {
	_, err := JMESPath(string(text)).expr()
	return err
}

// MarshalText implements encoding.TextMarshaler.
func (j JMESPath) MarshalText() (text []byte, err error) {
	return []byte(j), nil
}

func (j JMESPath) expr() (jmespath.JMESPath, error) {
	expr, err := jmespath.Compile(string(j))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCouldNotCompileJMES, err)
	}
	return expr, nil
}
