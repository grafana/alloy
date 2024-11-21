package ssh

import (
    "errors"

    "github.com/grafana/alloy/internal/component"
    "github.com/grafana/alloy/internal/component/prometheus/exporter"
    "github.com/grafana/alloy/internal/featuregate"
    "github.com/grafana/alloy/internal/static/integrations"
    "github.com/grafana/alloy/internal/static/integrations/ssh_exporter"
)

func init() {
    component.Register(component.Registration{
        Name:      "prometheus.exporter.ssh",
        Stability: featuregate.StabilityExperimental,
        Args:      Arguments{},
        Exports:   exporter.Exports{},
        Build:     exporter.New(createExporter, "ssh"),
    })
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
    a := args.(Arguments)
    return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

type Arguments struct {
    VerboseLogging bool     `alloy:"verbose_logging,attr,optional"`
    Targets        []Target `alloy:"targets,block"`
}

func (a *Arguments) Validate() error {
    if len(a.Targets) == 0 {
        return errors.New("at least one target must be specified")
    }
    for _, target := range a.Targets {
        if err := target.Validate(); err != nil {
            return err
        }
    }
    return nil
}

func (a *Arguments) Convert() *ssh_exporter.Config {
    targets := make([]ssh_exporter.Target, len(a.Targets))
    for i, t := range a.Targets {
        targets[i] = t.Convert()
    }
    return &ssh_exporter.Config{
        VerboseLogging: a.VerboseLogging,
        Targets:        targets,
    }
}

type Target struct {
    Address        string         `alloy:"address,attr"`
    Port           int            `alloy:"port,attr,optional"`
    Username       string         `alloy:"username,attr,optional"`
    Password       string         `alloy:"password,attr,optional"`
    KeyFile        string         `alloy:"key_file,attr,optional"`
    CommandTimeout int            `alloy:"command_timeout,attr,optional"`
    CustomMetrics  []CustomMetric `alloy:"custom_metrics,block,optional"`
}

func (t *Target) Validate() error {
    if t.Address == "" {
        return errors.New("target address cannot be empty")
    }
    if t.Port <= 0 || t.Port > 65535 {
        return errors.New("invalid port")
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

func (t *Target) Convert() ssh_exporter.Target {
    customMetrics := make([]ssh_exporter.CustomMetric, len(t.CustomMetrics))
    for i, cm := range t.CustomMetrics {
        customMetrics[i] = cm.Convert()
    }
    return ssh_exporter.Target{
        Address:        t.Address,
        Port:           t.Port,
        Username:       t.Username,
        Password:       t.Password,
        KeyFile:        t.KeyFile,
        CommandTimeout: t.CommandTimeout,
        CustomMetrics:  customMetrics,
    }
}

type CustomMetric struct {
    Name       string            `alloy:"name,attr"`
    Command    string            `alloy:"command,attr"`
    Type       string            `alloy:"type,attr"`
    Help       string            `alloy:"help,attr,optional"`
    Labels     map[string]string `alloy:"labels,attr,optional"`
    ParseRegex string            `alloy:"parse_regex,attr,optional"`
}

func (cm *CustomMetric) Validate() error {
    if cm.Name == "" {
        return errors.New("custom metric name cannot be empty")
    }
    if cm.Command == "" {
        return errors.New("custom metric command cannot be empty")
    }
    if cm.Type != "gauge" && cm.Type != "counter" {
        return errors.New("unsupported metric type")
    }
    return nil
}

func (cm *CustomMetric) Convert() ssh_exporter.CustomMetric {
    return ssh_exporter.CustomMetric{
        Name:       cm.Name,
        Command:    cm.Command,
        Type:       cm.Type,
        Help:       cm.Help,
        Labels:     cm.Labels,
        ParseRegex: cm.ParseRegex,
    }
}
