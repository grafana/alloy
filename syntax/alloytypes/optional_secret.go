package alloytypes

import (
	"fmt"

	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
)

// OptionalSecret holds a potentially sensitive value. When IsSecret is true,
// the OptionalSecret's Value will be treated as sensitive and will be hidden
// from users when rendering Alloy syntax.
//
// OptionalSecrets may be converted from Alloy syntax strings and the Secret
// type, which will set IsSecret accordingly.
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

// AlloyCapsule marks OptionalSecret as a AlloyCapsule.
func (s OptionalSecret) AlloyCapsule() {}

// ConvertInto converts the OptionalSecret and stores it into the Go value
// pointed at by dst. OptionalSecrets can always be converted into *Secret.
// OptionalSecrets can only be converted into *string if IsSecret is false. In
// other cases, this method will return an explicit error or
// syntax.ErrNoConversion.
func (s OptionalSecret) ConvertInto(dst any) error {
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
// cases, this method will return syntax.ErrNoConversion.
func (s *OptionalSecret) ConvertFrom(src any) error {
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

// AlloyTokenize returns a set of custom tokens to represent this value in
// Alloy syntax text.
func (s OptionalSecret) AlloyTokenize() []builder.Token {
	if s.IsSecret {
		return []builder.Token{{Tok: token.LITERAL, Lit: "(secret)"}}
	}
	return []builder.Token{{
		Tok: token.STRING,
		Lit: fmt.Sprintf("%q", s.Value),
	}}
}
