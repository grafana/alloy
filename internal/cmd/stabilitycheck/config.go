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
	seen := make(map[string]bool, len(c.Components))
	for i, e := range c.Components {
		if e.Name == "" {
			return fmt.Errorf("components[%d]: name is required", i)
		}
		if seen[e.Name] {
			return fmt.Errorf("components[%d]: duplicate name %q", i, e.Name)
		}
		seen[e.Name] = true
		// Entries must stay sorted by name so the file is easy to scan and edit.
		if i > 0 && e.Name < c.Components[i-1].Name {
			return fmt.Errorf("components[%d] (%s): entries must be sorted by name; %q must come before %q", i, e.Name, e.Name, c.Components[i-1].Name)
		}
		if !validStability[e.Stability] {
			return fmt.Errorf("components[%d] (%s): stability must be one of experimental or public-preview, got %q", i, e.Name, e.Stability)
		}
		if e.Reason == "" {
			return fmt.Errorf("components[%d] (%s): reason is required", i, e.Name)
		}
		if e.Reason == placeholderReason {
			return fmt.Errorf("components[%d] (%s): reason is still the %q placeholder — write a real justification", i, e.Name, placeholderReason)
		}
		if e.Expires.IsZero() {
			return fmt.Errorf("components[%d] (%s): expires is required (YYYY-MM-DD)", i, e.Name)
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
