package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
)

type MimirRuleGroup struct {
	rulefmt.RuleGroup `yaml:",inline"`
	// Source tenants is extracted from the CR annotations and not the actual rule group definition.
	SourceTenants []string `yaml:"source_tenants,omitempty"`
}

type mimirRuleGroups struct {
	Groups []yaml.Node `yaml:"groups"`
}

type MimirRuleGroups struct {
	Groups []MimirRuleGroup `yaml:"groups"`
}

// Validate validates all rules in the rule groups.
func (g *MimirRuleGroups) Validate(node mimirRuleGroups) (errs []error) {
	set := map[string]struct{}{}

	for j, g := range g.Groups {
		if g.Name == "" {
			errs = append(errs, fmt.Errorf("%d:%d: Groupname must not be empty", node.Groups[j].Line, node.Groups[j].Column))
		}

		if _, ok := set[g.Name]; ok {
			errs = append(
				errs,
				fmt.Errorf("%d:%d: groupname: \"%s\" is repeated in the same file", node.Groups[j].Line, node.Groups[j].Column, g.Name),
			)
		}

		set[g.Name] = struct{}{}

		for i, r := range g.Rules {
			for _, node := range g.Rules[i].Validate() {
				var ruleName yaml.Node
				if r.Alert.Value != "" {
					ruleName = r.Alert
				} else {
					ruleName = r.Record
				}
				errs = append(errs, &rulefmt.Error{
					Group:    g.Name,
					Rule:     i + 1,
					RuleName: ruleName.Value,
					Err:      node,
				})
			}
		}
	}

	return errs
}

// Parse parses and validates a set of rules in the mimir format (supporting source_tenants).
// This was copied and adjusted from the rulefmt package to support the mimir format.
func Parse(content []byte) (*MimirRuleGroups, []error) {
	var (
		groups MimirRuleGroups
		node   mimirRuleGroups
		errs   []error
	)

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	err := decoder.Decode(&groups)
	// Ignore io.EOF which happens with empty input.
	if err != nil && !errors.Is(err, io.EOF) {
		errs = append(errs, err)
	}
	err = yaml.Unmarshal(content, &node)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return &groups, groups.Validate(node)
}
