package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Ignore []IgnoreEntry `yaml:"ignore"`
}

// IgnoreEntry suppresses one OSV ID. Reason is mandatory so the rationale
// lives in git; the optional Expires forces re-evaluation when it passes.
type IgnoreEntry struct {
	ID      string    `yaml:"id"`
	Reason  string    `yaml:"reason"`
	Expires time.Time `yaml:"expires,omitempty"` // YAML accepts YYYY-MM-DD or RFC3339
}

var goIDRegexp = regexp.MustCompile(`^GO-\d{4}-\d+$`)

// loadConfig reads path. A missing file is treated as an empty config.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseConfig(data)
}

func parseConfig(data []byte) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // reject unknown fields so typos fail loud
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	seen := make(map[string]bool, len(c.Ignore))
	for i, e := range c.Ignore {
		if !goIDRegexp.MatchString(e.ID) {
			return fmt.Errorf("ignore[%d]: id %q is not a valid GO-YYYY-NNNN identifier", i, e.ID)
		}
		if seen[e.ID] {
			return fmt.Errorf("ignore[%d]: duplicate id %q", i, e.ID)
		}
		seen[e.ID] = true
		if e.Reason == "" {
			return fmt.Errorf("ignore[%d] (%s): reason is required", i, e.ID)
		}
	}
	return nil
}

// isIgnored returns the matching entry, or nil if not ignored or expired.
func (c *Config) isIgnored(id string, now time.Time) *IgnoreEntry {
	for i := range c.Ignore {
		e := &c.Ignore[i]
		if e.ID != id {
			continue
		}
		if !e.Expires.IsZero() && now.After(e.Expires) {
			return nil
		}
		return e
	}
	return nil
}
