package statictest

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Recipe rule scopes.
const (
	ScopeAlloy      = "alloy"
	ScopeDeployment = "deployment"
)

// RecipeRules is the rules.yaml file of a recipe.
type RecipeRules struct {
	Rules []RecipeRule `yaml:"rules"`
}

// RecipeRule is a rule of a recipe. Static is the check a static test copies.
// A rule without it is for the verifier agent.
type RecipeRule struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Docs        string `yaml:"docs,omitempty"`
	Severity    string `yaml:"severity"`
	Scope       string `yaml:"scope"`
	Static      *Check `yaml:"static,omitempty"`
}

// LoadRecipeRules reads and checks a recipe's rules.yaml.
func LoadRecipeRules(path string) (RecipeRules, error) {
	bb, err := os.ReadFile(path)
	if err != nil {
		return RecipeRules{}, err
	}
	var rr RecipeRules
	dec := yaml.NewDecoder(bytes.NewReader(bb))
	dec.KnownFields(true)
	if err := dec.Decode(&rr); err != nil {
		return RecipeRules{}, err
	}
	for _, r := range rr.Rules {
		if r.Name == "" || r.Description == "" {
			return RecipeRules{}, fmt.Errorf("every rule needs a name and a description")
		}
		if r.Severity != SeverityMust && r.Severity != SeverityShould {
			return RecipeRules{}, fmt.Errorf("rule %q: severity must be must or should", r.Name)
		}
		if r.Scope != ScopeAlloy && r.Scope != ScopeDeployment {
			return RecipeRules{}, fmt.Errorf("rule %q: scope must be alloy or deployment", r.Name)
		}
		if r.Static == nil {
			continue
		}
		if r.Scope != ScopeAlloy {
			return RecipeRules{}, fmt.Errorf("rule %q: only alloy rules can have a static check", r.Name)
		}
		if err := r.Static.Validate(); err != nil {
			return RecipeRules{}, fmt.Errorf("rule %q: %w", r.Name, err)
		}
	}
	return rr, nil
}

// TestRule returns the rule a static test holds for a recipe rule with a
// static check.
func (r RecipeRule) TestRule() Rule {
	return Rule{
		Name:        r.Name,
		Description: r.Description,
		Docs:        r.Docs,
		Severity:    r.Severity,
		Check:       *r.Static,
	}
}
