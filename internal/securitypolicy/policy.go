// Package securitypolicy implements operator-defined restrictions on which
// Alloy components and features may be used at runtime.
package securitypolicy

import (
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// PolicySection defines a single gate as either an allowlist or a denylist.
// When Mode is empty the section is absent and all names are permitted.
type PolicySection struct {
	// Mode is either "allowlist" or "denylist".
	Mode string `yaml:"mode"`
	// List is the set of names the mode applies to.
	List []string `yaml:"list"`
}

func (s PolicySection) check(kind, name string) error {
	switch s.Mode {
	case "":
		return nil
	case "allowlist":
		if !slices.Contains(s.List, name) {
			return fmt.Errorf("%s %q is not in the security policy allowlist", kind, name)
		}
	case "denylist":
		if slices.Contains(s.List, name) {
			return fmt.Errorf("%s %q is in the security policy denylist", kind, name)
		}
	default:
		return fmt.Errorf("unknown security policy mode %q for %s", s.Mode, kind)
	}
	return nil
}

// SecurityPolicy is the top-level policy document.
// A nil *SecurityPolicy permits everything.
type SecurityPolicy struct {
	Components PolicySection `yaml:"components"`
}

// CheckComponent returns an error if name is not permitted by the components section.
func (p *SecurityPolicy) CheckComponent(name string) error {
	if p == nil {
		return nil
	}
	return p.Components.check("component", name)
}

// LoadFromFile reads and parses a YAML security policy from path.
// Returns nil, nil when path is empty (no policy configured).
func LoadFromFile(path string) (*SecurityPolicy, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading security policy file: %w", err)
	}
	var p SecurityPolicy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing security policy file: %w", err)
	}
	return &p, nil
}
