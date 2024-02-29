// Package features enables a way to encode enabled features in a
// flag.FlagSet.
package features

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

// Feature is an experimental feature. Features are case-insensitive.
type Feature string

const setFlagName = "enable-features"

// Register sets a flag in fs to track enabled features. The list of possible
// features is enumerated by ff. ff must contain a unique set of case-insensitive
// features. Register will panic if ff is invalid.
func Register(fs *flag.FlagSet, ff []Feature) {
	var (
		cache = make(map[Feature]struct{}, len(ff))
		names = make([]string, len(ff))
	)
	for i, f := range ff {
		normalized := normalize(f)
		if _, found := cache[normalized]; found {
			panic(fmt.Sprintf("case-insensitive feature %q registered twice", normalized))
		}
		cache[normalized] = struct{}{}
		names[i] = string(normalized)
	}

	help := fmt.Sprintf("Comma-delimited list of features to enable. Valid values: %s", strings.Join(names, ", "))

	s := set{valid: cache, validString: strings.Join(names, ", ")}
	fs.Var(&s, setFlagName, help)
}

func normalize(f Feature) Feature {
	return Feature(strings.ToLower(string(f)))
}

// Enabled returns true if a feature is enabled. Enable will panic if fs has
// not been passed to Register or name is an unknown feature.
func Enabled(fs *flag.FlagSet, name Feature) bool {
	name = normalize(name)

	f := fs.Lookup(setFlagName)
	if f == nil {
		panic("feature flag not registered to fs")
	}
	s, ok := f.Value.(*set)
	if !ok {
		panic("registered feature flag not appropriate type")
	}

	if _, valid := s.valid[name]; !valid {
		panic(fmt.Sprintf("unknown feature %q", name))
	}
	_, enabled := s.enabled[name]
	return enabled
}

// Dependency marks a Flag as depending on a specific feature being enabled.
type Dependency struct {
	// Flag must be a flag name from a FlagSet.
	Flag string
	// Feature which must be enabled for Flag to be provided at the command line.
	Feature Feature
}

// Validate returns an error if any flags from deps were used without the
// corresponding feature being enabled.
//
// If deps references a flag that is not in fs, Validate will panic.
func Validate(fs *flag.FlagSet, deps []Dependency) error {
	depLookup := make(map[string]Dependency, len(deps))

	for _, dep := range deps {
		if fs.Lookup(dep.Flag) == nil {
			panic(fmt.Sprintf("flag %q does not exist in fs", dep.Flag))
		}
		depLookup[dep.Flag] = dep

		// Ensure that the feature also exists. We ignore the result here;
		// we just want to propagate the panic behavior.
		_ = Enabled(fs, dep.Feature)
	}

	var err error

	// Iterate over all the flags that were passed at the command line.
	// Flags that were passed and are present in deps MUST also have their
	// corresponding feature enabled.
	fs.Visit(func(f *flag.Flag) {
		// If we have an error to return, stop iterating.
		if err != nil {
			return
		}

		dep, ok := depLookup[f.Name]
		if !ok {
			return
		}

		// Flag was provided and exists in deps.
		if !Enabled(fs, dep.Feature) {
			err = fmt.Errorf("flag %q requires feature %q to be provided in --%s", f.Name, dep.Feature, setFlagName)
		}
	})

	return err
}

// GetAllEnabled returns the list of all enabled features
func GetAllEnabled(fs *flag.FlagSet) []string {
	f := fs.Lookup(setFlagName)
	if f == nil {
		panic("feature flag not registered to fs")
	}
	s, ok := f.Value.(*set)
	if !ok {
		panic("registered feature flag not appropriate type")
	}
	var enabled []string
	for feature := range s.enabled {
		enabled = append(enabled, string(feature))
	}
	return enabled
}

// set implements flag.Value and holds the set of enabled features.
// set should be provided to a flag.FlagSet with:
//
//	var s features.set
//	fs.Var(&s, features.SetFlag, "")
type set struct {
	valid       map[Feature]struct{}
	validString string // Comma-delimited list of acceptable values

	enabled map[Feature]struct{}
}

// Set implements flag.Value.
func (s *set) String() string {
	res := make([]string, 0, len(s.enabled))
	for k := range s.enabled {
		res = append(res, string(k))
	}
	sort.Strings(res)
	return strings.Join(res, ",")
}

// Set implements flag.Value.
func (s *set) Set(in string) error {
	slice := strings.Split(in, ",")

	m := make(map[Feature]struct{}, len(slice))
	for _, input := range slice {
		f := normalize(Feature(input))
		if _, valid := s.valid[f]; !valid {
			return fmt.Errorf("unknown feature %q. possible options: %s", f, s.validString)
		} else if _, ok := m[f]; ok {
			return fmt.Errorf("%q already set", f)
		}
		m[f] = struct{}{}
	}

	s.enabled = m
	return nil
}
