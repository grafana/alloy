package ssh_exporter

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var DefaultConfig = Config{
	Targets: []Target{},
}

// Config defines the configuration for the SSH exporter
type Config struct {
	// Targets is the list of SSH targets to collect metrics from
	Targets []Target `yaml:"targets,omitempty"`
}

type Target struct {
	// SkipAuth, when true, disables authentication enforcement (for dynamic use-cases)
	SkipAuth       bool           `yaml:"-"`
	Address        string         `yaml:"address"`
	Port           int            `yaml:"port"`
	Username       string         `yaml:"username"`
	Password       string         `yaml:"password,omitempty"`
	KeyFile        string         `yaml:"key_file,omitempty"`
	CommandTimeout time.Duration  `yaml:"command_timeout,omitempty"`
	CustomMetrics  []CustomMetric `yaml:"custom_metrics,omitempty"`
}

func (t *Target) Validate() error {
	if t.Address == "" {
		return errors.New("target address cannot be empty")
	}
	// Validate that address is a valid IP or hostname
	if _, err := url.ParseRequestURI("ssh://" + t.Address); err != nil {
		return fmt.Errorf("invalid address: %s: %w", t.Address, err)
	}
	if t.Port <= 0 || t.Port > 65535 {
		return fmt.Errorf("invalid port: %d", t.Port)
	}
	if t.Username == "" {
		return errors.New("username cannot be empty")
	}
	for _, cm := range t.CustomMetrics {
		if err := cm.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type CustomMetric struct {
	Name       string            `yaml:"name"`
	Command    string            `yaml:"command"`
	Type       string            `yaml:"type"`
	Help       string            `yaml:"help"`
	Labels     map[string]string `yaml:"labels,omitempty"`
	ParseRegex string            `yaml:"parse_regex,omitempty"`
}

func (cm *CustomMetric) Validate() error {
	if cm.Name == "" {
		return errors.New("custom metric name cannot be empty")
	}
	if cm.Command == "" {
		return errors.New("custom metric command cannot be empty")
	}
	// Disallow potentially unsafe shell characters in command
	// Disallow backticks and semicolons to prevent command injection
	if strings.ContainsAny(cm.Command, "`;") {
		return fmt.Errorf("custom metric command contains unsafe characters")
	}
	if cm.Type != "gauge" && cm.Type != "counter" {
		return fmt.Errorf("unsupported metric type: %s", cm.Type)
	}
	return nil
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

func (c *Config) Validate() error {
	if len(c.Targets) == 0 {
		return errors.New("at least one target must be specified")
	}
	for _, target := range c.Targets {
		if err := target.Validate(); err != nil {
			return err
		}
	}
	return nil
}
