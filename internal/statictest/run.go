package statictest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Result is the outcome of a static test that ran.
type Result struct {
	Rules []RuleResult
}

// Passed reports whether every must rule passed. A failed should rule is a
// warning.
func (r Result) Passed() bool {
	for _, rr := range r.Rules {
		if !rr.Passed && rr.Rule.Severity != SeverityShould {
			return false
		}
	}
	return true
}

// LoadTestFile reads a static test file. The caller checks kind and version
// first. Unknown fields are rejected, so a typo fails the test.
func LoadTestFile(path string) (TestSchema, error) {
	bb, err := os.ReadFile(path)
	if err != nil {
		return TestSchema{}, err
	}
	var schema TestSchema
	dec := yaml.NewDecoder(bytes.NewReader(bb))
	dec.KnownFields(true)
	if err := dec.Decode(&schema); err != nil {
		return TestSchema{}, err
	}
	if schema.Config.Path == "" {
		return TestSchema{}, errors.New("config.path is required")
	}
	if len(schema.Rules) == 0 {
		return TestSchema{}, errors.New("the test has no rules")
	}
	for _, r := range schema.Rules {
		if err := r.validate(); err != nil {
			return TestSchema{}, err
		}
	}
	return schema, nil
}

// RunFile loads the static test at path and checks its config, which is
// relative to the test file.
func RunFile(path string) (Result, error) {
	schema, err := LoadTestFile(path)
	if err != nil {
		return Result{}, err
	}
	configPath := schema.Config.Path
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(filepath.Dir(path), configPath)
	}
	src, err := os.ReadFile(configPath)
	if err != nil {
		return Result{}, fmt.Errorf("read config: %w", err)
	}
	return Run(schema.Config.Path, src, schema.Rules)
}

// Run checks the rules against an Alloy config. name labels the config in
// messages.
func Run(name string, src []byte, rules []Rule) (Result, error) {
	c, err := parseConfig(name, src)
	if err != nil {
		return Result{}, err
	}
	var res Result
	for _, r := range rules {
		res.Rules = append(res.Rules, checkRule(c, r))
	}
	return res, nil
}
