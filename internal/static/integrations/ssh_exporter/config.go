package ssh_exporter

import (
    "errors"
    "fmt"
)

var DefaultConfig = Config{
    VerboseLogging: false,
    Targets:        []Target{},
}

type Config struct {
    VerboseLogging bool     `yaml:"verbose_logging,omitempty"`
    Targets        []Target `yaml:"targets,omitempty"`
}

type Target struct {
    Address        string         `yaml:"address"`
    Port           int            `yaml:"port"`
    Username       string         `yaml:"username"`
    Password       string         `yaml:"password,omitempty"`
    KeyFile        string         `yaml:"key_file,omitempty"`
    CommandTimeout int            `yaml:"command_timeout,omitempty"`
    CustomMetrics  []CustomMetric `yaml:"custom_metrics,omitempty"`
}

func (t *Target) Validate() error {
    if t.Address == "" {
        return errors.New("target address cannot be empty")
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
