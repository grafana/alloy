package util

import "flag"

// DefaultConfigFromFlags will load default values into cfg by
// retrieving default values that are registered as flags.
//
// cfg must implement either PrefixedConfigFlags or ConfigFlags.
func DefaultConfigFromFlags(cfg interface{}) interface{} {
	// This function is super ugly but is required for mixing the combination
	// of mechanisms for providing default for config structs that are used
	// across both Prometheus (via UnmarshalYAML and assigning the default object)
	// and Cortex (via RegisterFlags*).
	//
	// The issue stems from default values assigned via RegisterFlags being set
	// at *registration* time, not *flag parse* time. For example, this
	// flag:
	//
	//   fs.BoolVar(&enabled, "enabled", true, "enable everything")
	//
	// Sets enabled to true as soon as fs.BoolVar is called. Normally this is
	// fine, but with how Prometheus implements UnmarshalYAML, these defaults
	// get overridden:
	//
	//   func (c *Config) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	//     *c = DefaultConfig // <-- !! overrides defaults from flags !!
	//     type plain Config
	//     return unmarshal((*plain)(c))
	//   }
	//
	// The solution to this is to make sure that the DefaultConfig object contains
	// the defaults that are set up through registering flags. Unfortunately, the
	// best way to do this is this function that creates a temporary flagset just for
	// the sake of collecting default values.
	//
	// This function should be used like so:
	//
	//   var DefaultConfig = *DefaultConfigFromFlags(&Config{}).(*Config)

	fs := flag.NewFlagSet("DefaultConfigFromFlags", flag.PanicOnError)

	if v, ok := cfg.(PrefixedConfigFlags); ok {
		v.RegisterFlagsWithPrefix("", fs)
	} else if v, ok := cfg.(ConfigFlags); ok {
		v.RegisterFlags(fs)
	} else {
		panic("config does not implement PrefixedConfigFlags or ConfigFlags")
	}

	return cfg
}

// ConfigFlags is an interface that will register flags that can control
// some object.
type ConfigFlags interface {
	RegisterFlags(f *flag.FlagSet)
}

// PrefixedConfigFlags is an interface that, given a prefix for flags
// and a flagset, will register flags that can control some object.
type PrefixedConfigFlags interface {
	RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet)
}
