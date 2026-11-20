package otelgen

// ExtraArguments adds arguments that only exist in Alloy. The package defines
// them by hand in extra.go, in an ExtraArguments struct squashed into
// Arguments:
//
//	func (e *ExtraArguments) SetToDefault()
//	func (e ExtraArguments) ApplyTo(cfg *<upstream>.Config) error
//
// SetToDefault runs after the generated defaults and ApplyTo at the end of
// Convert, after the generated conversions, so it can change any upstream
// field.
func ExtraArguments() Option {
	return func(c *Component) { c.extraArguments = true }
}

// ExtraValidation adds validation that upstream does not do. The package
// defines it by hand in extra.go:
//
//	func (args *Arguments) validate() error
//
// Validate runs it before converting and calling upstream validation.
func ExtraValidation() Option {
	return func(c *Component) { c.extraValidation = true }
}
