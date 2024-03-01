package rivertypes

import (
	"fmt"

	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/token"
	"github.com/grafana/river/token/builder"
)

// Secret is a River capsule holding a sensitive string. The contents of a
// Secret are never displayed to the user when rendering River.
//
// Secret allows itself to be converted from a string River value, but never
// the inverse. This ensures that a user can't accidentally leak a sensitive
// value.
type Secret string

var (
	_ value.Capsule                = Secret("")
	_ value.ConvertibleIntoCapsule = Secret("")
	_ value.ConvertibleFromCapsule = (*Secret)(nil)

	_ builder.Tokenizer = Secret("")
)

// RiverCapsule marks Secret as a RiverCapsule.
func (s Secret) RiverCapsule() {}

// ConvertInto converts the Secret and stores it into the Go value pointed at
// by dst. Secrets can be converted into *OptionalSecret. In other cases, this
// method will return an explicit error or river.ErrNoConversion.
func (s Secret) ConvertInto(dst interface{}) error {
	switch dst := dst.(type) {
	case *OptionalSecret:
		*dst = OptionalSecret{IsSecret: true, Value: string(s)}
		return nil
	case *string:
		return fmt.Errorf("secrets may not be converted into strings")
	}

	return value.ErrNoConversion
}

// ConvertFrom converts the src value and stores it into the Secret s.
// OptionalSecrets and strings can be converted into a Secret. In other cases,
// this method will return river.ErrNoConversion.
func (s *Secret) ConvertFrom(src interface{}) error {
	switch src := src.(type) {
	case OptionalSecret:
		*s = Secret(src.Value)
		return nil
	case string:
		*s = Secret(src)
		return nil
	}

	return value.ErrNoConversion
}

// RiverTokenize returns a set of custom tokens to represent this value in
// River text.
func (s Secret) RiverTokenize() []builder.Token {
	return []builder.Token{{Tok: token.LITERAL, Lit: "(secret)"}}
}
