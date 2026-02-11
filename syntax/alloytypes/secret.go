package alloytypes

import (
	"fmt"

	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
)

// Secret is an Alloy syntax capsule holding a sensitive string. The contents
// of a Secret are never displayed to the user when rendering Alloy
// configuration.
//
// Secret allows itself to be converted from a string Alloy syntax value, but
// never the inverse. This ensures that a user can't accidentally leak a
// sensitive value.
type Secret string

var (
	_ value.Capsule                = Secret("")
	_ value.ConvertibleIntoCapsule = Secret("")
	_ value.ConvertibleFromCapsule = (*Secret)(nil)

	_ builder.Tokenizer = Secret("")
)

// AlloyCapsule marks Secret as a AlloyCapsule.
func (s Secret) AlloyCapsule() {}

// ConvertInto converts the Secret and stores it into the Go value pointed at
// by dst. Secrets can be converted into *OptionalSecret. In other cases, this
// method will return an explicit error or syntax.ErrNoConversion.
func (s Secret) ConvertInto(dst any) error {
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
// this method will return syntax.ErrNoConversion.
func (s *Secret) ConvertFrom(src any) error {
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

// AlloyTokenize returns a set of custom tokens to represent this value in
// Alloy syntax text.
func (s Secret) AlloyTokenize() []builder.Token {
	return []builder.Token{{Tok: token.LITERAL, Lit: "(secret)"}}
}
