// Package featuregate provides a way to gate features in the collector based on different options, such as the
// feature's stability level and user-defined minimum allowed stability level.
package featuregate

import (
	"fmt"

	"github.com/spf13/pflag"
)

// Stability is used to designate the stability level of a feature or a minimum stability level the collector
// is allowed to operate with.
type Stability int

const (
	// StabilityUndefined is the default value for Stability, which indicates an error and should never be used.
	StabilityUndefined Stability = iota
	// StabilityExperimental is used to designate experimental features.
	StabilityExperimental
	// StabilityBeta is used to designate beta features.
	StabilityBeta
	// StabilityStable is used to designate stable features.
	StabilityStable
)

func CheckAllowed(stability Stability, minStability Stability, featureName string) error {
	if stability == StabilityUndefined || minStability == StabilityUndefined {
		return fmt.Errorf(
			"stability levels must be defined: got %s as stability of %s and %s as the minimum stability level",
			stability,
			featureName,
			minStability,
		)
	}
	if stability < minStability {
		return fmt.Errorf(
			"%s is at stability level %s, which is below the minimum allowed stability level %s. "+
				"Use --stability.level command-line flag to enable %s features",
			featureName,
			stability,
			minStability,
			stability,
		)
	}
	return nil
}

func AllowedValues() []string {
	return []string{
		StabilityStable.String(),
		StabilityBeta.String(),
		StabilityExperimental.String(),
	}
}

var (
	// Stability implements the pflag.Value interface for use with Cobra flags.
	_ pflag.Value = (*Stability)(nil)
	// stabilityToString defines how to convert a Stability to a string.
	stabilityToString = map[Stability]string{
		StabilityExperimental: "experimental",
		StabilityBeta:         "beta",
		StabilityStable:       "stable",
	}
)

// String implements the pflag.Value interface. The returned strings are "double-quoted" already.
func (s Stability) String() string {
	if str, ok := stabilityToString[s]; ok {
		return fmt.Sprintf("%q", str)
	}
	return "<invalid_stability_level>"
}

// Set implements the pflag.Value interface.
func (s *Stability) Set(str string) error {
	for k, v := range stabilityToString {
		if v == str {
			*s = k
			return nil
		}
	}
	return fmt.Errorf("invalid stability level %q", str)
}

// Type implements the pflag.Value interface. This value is displayed as a placeholder in help messages.
func (s Stability) Type() string {
	return "<stability_level>"
}
