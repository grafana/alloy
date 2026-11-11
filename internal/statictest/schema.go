// Package statictest checks an Alloy configuration against static rules,
// without running it. Refer to README.md for the file format.
package statictest

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

const (
	// Kind is the value of the kind field in every static test file.
	Kind = "statictest"
	// Version is the static test file format version this Alloy reads.
	Version = 1
)

// Rule types.
const (
	TypeAttribute    = "attribute"
	TypeAllOrNone    = "all-or-none"
	TypeCount        = "count"
	TypeFlowsThrough = "flows-through"
)

// Severities. A failed should rule is a warning, and doesn't fail the test.
const (
	SeverityMust   = "must"
	SeverityShould = "should"
)

// TestSchema is a static test file.
type TestSchema struct {
	Kind    string       `yaml:"kind"`
	Version int          `yaml:"version"`
	Config  ConfigSchema `yaml:"config"`
	Rules   []Rule       `yaml:"rules"`
}

// ConfigSchema names the Alloy configuration under test.
type ConfigSchema struct {
	// Path is relative to the test file.
	Path string `yaml:"path"`
}

// Rule is one static rule: what it's called and why it exists, and the check.
type Rule struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Docs        string `yaml:"docs,omitempty"`
	// Severity is must, the default, or should.
	Severity string `yaml:"severity,omitempty"`
	Check    `yaml:",inline"`
}

// Check is the part of a rule that the checker runs. Recipes store it on its
// own, so a test file can copy it.
type Check struct {
	Type string `yaml:"type"`

	// Select picks components by name, such as prometheus.scrape, by ID,
	// such as prometheus.scrape.default, or by a pattern with *.
	Select Selector `yaml:"select,omitempty"`
	// Path is a dot-separated path inside a component, such as
	// clustering.enabled or endpoint[*].url.
	Path string `yaml:"path,omitempty"`

	// Equals and Matches are the conditions of attribute and all-or-none.
	Equals  *any   `yaml:"equals,omitempty"`
	Matches string `yaml:"matches,omitempty"`

	// Min and Max bound a count.
	Min *int `yaml:"min,omitempty"`
	Max *int `yaml:"max,omitempty"`

	// From, To and Through are the components of flows-through.
	From    Selector `yaml:"from,omitempty"`
	To      Selector `yaml:"to,omitempty"`
	Through Selector `yaml:"through,omitempty"`

	// When limits the rule to configs where the condition holds.
	When *Condition `yaml:"when,omitempty"`
}

// Condition is a count that must hold for a rule to apply.
type Condition struct {
	Select Selector `yaml:"select"`
	Path   string   `yaml:"path,omitempty"`
	Min    *int     `yaml:"min,omitempty"`
	Max    *int     `yaml:"max,omitempty"`
}

// Selector is a list of component patterns. YAML can give one string or a
// list.
type Selector []string

func (s *Selector) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*s = Selector{node.Value}
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*s = list
	return nil
}

// Validate checks that the check has the fields its type needs.
func (c Check) Validate() error {
	need := func(ok bool, msg string) error {
		if !ok {
			return fmt.Errorf("%s needs %s", c.Type, msg)
		}
		return nil
	}
	var err error
	switch c.Type {
	case TypeAttribute, TypeAllOrNone:
		if err = need(len(c.Select) > 0 && c.Path != "", "select and path"); err != nil {
			return err
		}
		if (c.Equals == nil) == (c.Matches == "") {
			return fmt.Errorf("%s needs one of equals or matches", c.Type)
		}
		if c.Matches != "" {
			if _, err := regexp.Compile(c.Matches); err != nil {
				return fmt.Errorf("matches: %w", err)
			}
		}
	case TypeCount:
		err = need(len(c.Select) > 0 && (c.Min != nil || c.Max != nil), "select, and min or max")
	case TypeFlowsThrough:
		err = need(len(c.From) > 0 && len(c.To) > 0 && len(c.Through) > 0, "from, to and through")
	case "":
		return fmt.Errorf("type is missing, use one of %s, %s, %s or %s", TypeAttribute, TypeAllOrNone, TypeCount, TypeFlowsThrough)
	default:
		return fmt.Errorf("unknown type %q, use one of %s, %s, %s or %s", c.Type, TypeAttribute, TypeAllOrNone, TypeCount, TypeFlowsThrough)
	}
	if err != nil {
		return err
	}
	if c.When != nil && (len(c.When.Select) == 0 || (c.When.Min == nil && c.When.Max == nil)) {
		return fmt.Errorf("when needs select, and min or max")
	}
	return nil
}

func (r Rule) validate() error {
	if r.Name == "" {
		return fmt.Errorf("a rule has no name")
	}
	switch r.Severity {
	case "", SeverityMust, SeverityShould:
	default:
		return fmt.Errorf("rule %q: severity must be must or should, got %q", r.Name, r.Severity)
	}
	if err := r.Check.Validate(); err != nil {
		return fmt.Errorf("rule %q: %w", r.Name, err)
	}
	return nil
}
