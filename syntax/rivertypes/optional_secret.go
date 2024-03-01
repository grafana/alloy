package rivertypes

import (
	"fmt"

	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/token"
	"github.com/grafana/river/token/builder"
)

// OptionalSecret holds a potentially sensitive value. When IsSecret is true,
// the OptionalSecret's Value will be treated as sensitive and will be hidden
// from users when rendering River.
//
// OptionalSecrets may be converted from river strings and the Secret type,
// which will set IsSecret accordingly.
//
// Additionally, OptionalSecrets may be converted into the Secret type
// regardless of the value of IsSecret. OptionalSecret can be converted into a
// string as long as IsSecret is false.
type OptionalSecret struct {
	IsSecret bool
	Value    string
}

var (
	_ value.Capsule                = OptionalSecret{}
	_ value.ConvertibleIntoCapsule = OptionalSecret{}
	_ value.ConvertibleFromCapsule = (*OptionalSecret)(nil)

	_ builder.Tokenizer = OptionalSecret{}
)

// RiverCapsule marks OptionalSecret as a RiverCapsule.
func (s OptionalSecret) RiverCapsule() {}

// ConvertInto converts the OptionalSecret and stores it into the Go value
// pointed at by dst. OptionalSecrets can always be converted into *Secret.
// OptionalSecrets can only be converted into *string if IsSecret is false. In
// other cases, this method will return an explicit error or
// river.ErrNoConversion.
func (s OptionalSecret) ConvertInto(dst interface{}) error {
	switch dst := dst.(type) {
	case *Secret:
		*dst = Secret(s.Value)
		return nil
	case *string:
		if s.IsSecret {
			return fmt.Errorf("secrets may not be converted into strings")
		}
		*dst = s.Value
		return nil
	}

	return value.ErrNoConversion
}

// ConvertFrom converts the src value and stores it into the OptionalSecret s.
// Secrets and strings can be converted into an OptionalSecret. In other
// cases, this method will return river.ErrNoConversion.
func (s *OptionalSecret) ConvertFrom(src interface{}) error {
	switch src := src.(type) {
	case Secret:
		*s = OptionalSecret{IsSecret: true, Value: string(src)}
		return nil
	case string:
		*s = OptionalSecret{Value: src}
		return nil
	}

	return value.ErrNoConversion
}

// RiverTokenize returns a set of custom tokens to represent this value in
// River text.
func (s OptionalSecret) RiverTokenize() []builder.Token {
	if s.IsSecret {
		return []builder.Token{{Tok: token.LITERAL, Lit: "(secret)"}}
	}
	return []builder.Token{{
		Tok: token.STRING,
		Lit: fmt.Sprintf("%q", s.Value),
	}}
}
