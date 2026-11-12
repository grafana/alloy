package statictest

import (
	"fmt"
	"strings"
)

// RuleResult is the outcome of one rule.
type RuleResult struct {
	Rule   Rule
	Passed bool
	// Message explains a failure, or why a rule passed without checking
	// anything, such as when no component matched.
	Message string
}

func checkRule(c *config, r Rule) RuleResult {
	res := RuleResult{Rule: r, Passed: true}
	fail := func(lines []string) RuleResult {
		res.Passed = false
		res.Message = strings.Join(lines, "\n")
		return res
	}

	if r.When != nil {
		n, err := countAt(c, r.When.Select, r.When.Path)
		if err != nil {
			return fail([]string{err.Error()})
		}
		if !inRange(n, r.When.Min, r.When.Max) {
			res.Message = "skipped: the when condition doesn't hold"
			return res
		}
	}

	var problems []string
	var err error
	switch r.Type {
	case TypeAttribute:
		problems, err = checkAttribute(c, r.Check)
	case TypeAllOrNone:
		problems, err = checkAllOrNone(c, r.Check)
	case TypeCount:
		problems, err = checkCount(c, r.Check)
	case TypeFlowsThrough:
		problems = checkFlowsThrough(c, r.Check)
	}
	if err != nil {
		return fail([]string{err.Error()})
	}
	if len(problems) > 0 {
		return fail(problems)
	}
	switch r.Type {
	case TypeAttribute, TypeAllOrNone:
		if len(c.selectComponents(r.Select)) == 0 {
			res.Message = "no component matches " + strings.Join(r.Select, ", ")
		}
	case TypeFlowsThrough:
		if len(c.selectComponents(r.From)) == 0 {
			res.Message = "no component matches from: " + strings.Join(r.From, ", ")
		} else if len(c.selectComponents(r.To)) == 0 {
			res.Message = "no component matches to: " + strings.Join(r.To, ", ")
		}
	}
	return res
}

// componentHolds reports whether every value at the path holds the
// condition. A path that isn't set doesn't hold.
func componentHolds(comp *component, p string, cond condition) (bool, string, error) {
	vals, err := resolve(comp.block, p)
	if err != nil {
		return false, "", err
	}
	if len(vals) == 0 {
		return false, p + " is not set", nil
	}
	for _, v := range vals {
		if v.attr == nil {
			return false, p + " is a block, not an attribute", nil
		}
		if ok, why := cond.holds(v.attr); !ok {
			return false, p + " " + why, nil
		}
	}
	return true, "", nil
}

// checkAttribute requires the condition on every selected component.
func checkAttribute(c *config, chk Check) ([]string, error) {
	cond := newCondition(chk)
	var problems []string
	for _, comp := range c.selectComponents(chk.Select) {
		ok, why, err := componentHolds(comp, chk.Path, cond)
		if err != nil {
			return nil, err
		}
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: %s, want %s", c.where(comp), why, cond))
		}
	}
	return problems, nil
}

// checkAllOrNone requires the condition on all selected components, or on
// none of them.
func checkAllOrNone(c *config, chk Check) ([]string, error) {
	cond := newCondition(chk)
	var with, without []string
	for _, comp := range c.selectComponents(chk.Select) {
		ok, why, err := componentHolds(comp, chk.Path, cond)
		if err != nil {
			return nil, err
		}
		if ok {
			with = append(with, c.where(comp))
		} else {
			without = append(without, c.where(comp)+": "+why)
		}
	}
	if len(with) == 0 || len(without) == 0 {
		return nil, nil
	}
	// The smaller group is the likely mistake, so list it in full and only
	// count the other.
	problems := []string{fmt.Sprintf("%s %s on %d components, but not on %d.", chk.Path, cond, len(with), len(without))}
	if len(without) <= len(with) {
		for _, w := range without {
			problems = append(problems, "  doesn't: "+w)
		}
	} else {
		for _, w := range with {
			problems = append(problems, "  has it: "+w)
		}
	}
	return problems, nil
}

// checkCount bounds how many components, or values at the path, there are.
func checkCount(c *config, chk Check) ([]string, error) {
	n, err := countAt(c, chk.Select, chk.Path)
	if err != nil {
		return nil, err
	}
	if inRange(n, chk.Min, chk.Max) {
		return nil, nil
	}
	what := strings.Join(chk.Select, ", ")
	if chk.Path != "" {
		what = chk.Path + " in " + what
	}
	problems := []string{fmt.Sprintf("found %d of %s, want %s", n, what, rangeText(chk.Min, chk.Max))}
	for _, comp := range c.selectComponents(chk.Select) {
		vals, err := resolve(comp.block, chk.Path)
		if err != nil {
			return nil, err
		}
		if len(vals) > 0 {
			problems = append(problems, "  "+c.where(comp))
		}
	}
	return problems, nil
}

func countAt(c *config, sel Selector, p string) (int, error) {
	n := 0
	for _, comp := range c.selectComponents(sel) {
		vals, err := resolve(comp.block, p)
		if err != nil {
			return 0, err
		}
		n += len(vals)
	}
	return n, nil
}

func inRange(n int, minVal, maxVal *int) bool {
	return (minVal == nil || n >= *minVal) && (maxVal == nil || n <= *maxVal)
}

func rangeText(minVal, maxVal *int) string {
	switch {
	case minVal != nil && maxVal != nil && *minVal == *maxVal:
		return fmt.Sprintf("exactly %d", *minVal)
	case minVal != nil && maxVal != nil:
		return fmt.Sprintf("%d to %d", *minVal, *maxVal)
	case minVal != nil:
		return fmt.Sprintf("at least %d", *minVal)
	default:
		return fmt.Sprintf("at most %d", *maxVal)
	}
}
