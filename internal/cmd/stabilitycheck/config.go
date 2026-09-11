package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// validStability lists the stability levels this tool tracks. GA is not tracked
// and community components have no stability, so neither is a valid entry value.
var validStability = map[string]bool{
	"experimental":   true,
	"public-preview": true,
}

// placeholderReason is the scaffold reason. CI must fail until a human replaces
// it, so it is rejected as an invalid reason.
const placeholderReason = "TODO"

// Config is the parsed .stability.yaml file.
type Config struct {
	Components []Entry `yaml:"components"`
	// Features tracks non-GA behavior that is not a component (config blocks,
	// stdlib functions, CLI features). This list is hand-maintained: there is no
	// registry to enumerate, so the tool cannot detect an untracked feature.
	Features []Entry `yaml:"features"`
}

// Entry justifies why one component is not yet generally available.
type Entry struct {
	Name      string    `yaml:"name"`
	Stability string    `yaml:"stability"`
	Reason    string    `yaml:"reason"`
	Expires   time.Time `yaml:"expires"` // YAML accepts YYYY-MM-DD or RFC3339.
}

// loadConfig reads path. A missing file is an empty config, so CI fails loudly
// on every non-GA component instead of passing silently.
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
	dec.KnownFields(true)
	// io.EOF means empty input (including comment-only YAML).
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if err := validateEntries(c.Components, "components"); err != nil {
		return err
	}
	return validateEntries(c.Features, "features")
}

// validateEntries checks one entry list. Names must be unique and sorted within
// the list, and every entry needs a valid non-GA stability, a real reason, and
// an expiry date.
func validateEntries(entries []Entry, section string) error {
	seen := make(map[string]bool, len(entries))
	for i, e := range entries {
		if e.Name == "" {
			return fmt.Errorf("%s[%d]: name is required", section, i)
		}
		if seen[e.Name] {
			return fmt.Errorf("%s[%d]: duplicate name %q", section, i, e.Name)
		}
		seen[e.Name] = true
		// Entries must stay sorted by name so the file is easy to scan and edit.
		if i > 0 && e.Name < entries[i-1].Name {
			return fmt.Errorf("%s[%d] (%s): entries must be sorted by name; %q must come before %q", section, i, e.Name, e.Name, entries[i-1].Name)
		}
		if !validStability[e.Stability] {
			return fmt.Errorf("%s[%d] (%s): stability must be one of experimental or public-preview, got %q", section, i, e.Name, e.Stability)
		}
		if e.Reason == "" {
			return fmt.Errorf("%s[%d] (%s): reason is required", section, i, e.Name)
		}
		if e.Reason == placeholderReason {
			return fmt.Errorf("%s[%d] (%s): reason is still the %q placeholder — write a real justification", section, i, e.Name, placeholderReason)
		}
		if e.Expires.IsZero() {
			return fmt.Errorf("%s[%d] (%s): expires is required (YYYY-MM-DD)", section, i, e.Name)
		}
	}
	return nil
}

// entryByName indexes entries by component name. Duplicates are rejected by
// validate, so each name maps to one entry.
func (c *Config) entryByName() map[string]*Entry {
	m := make(map[string]*Entry, len(c.Components))
	for i := range c.Components {
		m[c.Components[i].Name] = &c.Components[i]
	}
	return m
}
