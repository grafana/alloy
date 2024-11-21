package build

import (
    "github.com/grafana/alloy/internal/component/discovery"
    "github.com/grafana/alloy/internal/component/prometheus/exporter/ssh"
    "github.com/grafana/alloy/internal/static/integrations/ssh_exporter"
    "github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendSSHExporter(config *ssh_exporter.Config, instanceKey *string) discovery.Exports {
    args := toSSHExporter(config)
    return b.appendExporterBlock(args, config.Name(), instanceKey, "ssh")
}

func toSSHExporter(config *ssh_exporter.Config) *ssh.Arguments {
    targets := make([]ssh.Target, len(config.Targets))
    for i, t := range config.Targets {
        customMetrics := make([]ssh.CustomMetric, len(t.CustomMetrics))
        for j, cm := range t.CustomMetrics {
            customMetrics[j] = ssh.CustomMetric{
                Name:       cm.Name,
                Command:    cm.Command,
                Type:       cm.Type,
                Help:       cm.Help,
                Labels:     cm.Labels,
                ParseRegex: cm.ParseRegex,
            }
        }

        var password alloytypes.Secret
        if t.Password != "" {
            password = alloytypes.Secret(t.Password)
        }

        targets[i] = ssh.Target{
            Address:        t.Address,
            Port:           t.Port,
            Username:       t.Username,
            Password:       string(password),
            KeyFile:        t.KeyFile,
            CommandTimeout: t.CommandTimeout,
            CustomMetrics:  customMetrics,
        }
    }

    return &ssh.Arguments{
        VerboseLogging: config.VerboseLogging,
        Targets:        targets,
    }
}
