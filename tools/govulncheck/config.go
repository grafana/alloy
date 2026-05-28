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

// Config holds the parsed contents of .govulncheck.yaml.
type Config struct {
	Ignore []IgnoreEntry `yaml:"ignore"`
}

// IgnoreEntry suppresses a single OSV ID from failing the build. A reason is
// mandatory so that the rationale lives next to the entry in git. An optional
// expiry date forces periodic re-evaluation — once it passes, the entry stops
// applying and CI fails again until the entry is renewed or removed.
type IgnoreEntry struct {
	ID      string    `yaml:"id"`
	Reason  string    `yaml:"reason"`
	Expires time.Time `yaml:"expires,omitempty"` // YAML parses YYYY-MM-DD and RFC3339 natively
}

var goIDRegexp = regexp.MustCompile(`^GO-\d{4}-\d+$`)

// loadConfig reads the YAML file at path. A missing file is treated as an
// empty config (no ignores). An invalid file is a fatal error.
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

// isIgnored returns the matching IgnoreEntry for id, or nil if id is not
// ignored or its ignore has expired relative to now.
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
