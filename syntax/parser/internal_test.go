package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectFieldName(t *testing.T) {
	tt := []string{
		`field_a   = 5`,
		`"field_a" = 5`, // Quotes should be removed from the field name
	}

	for _, tc := range tt {
		p := newParser(t.Name(), []byte(tc))

		res := p.parseField()

		assert.Equal(t, "field_a", res.Name.Name)
	}
}
