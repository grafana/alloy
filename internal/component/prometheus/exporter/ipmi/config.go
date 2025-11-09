package ipmi

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/static/integrations/ipmi_exporter"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

// DefaultArguments holds non-zero default options for Arguments.
var DefaultArguments = Arguments{
	Timeout: 10 * time.Second,
}

// Arguments configures the prometheus.exporter.ipmi component.
type Arguments struct {
	// Local configures monitoring of the local machine's IPMI interface.
	Local LocalConfig `alloy:"local,block,optional"`

	// Targets to monitor via remote IPMI.
	Targets []IPMITarget `alloy:"target,block,optional"`

	// Timeout for IPMI requests.
	Timeout time.Duration `alloy:"timeout,attr,optional"`

	// ConfigFile points to an external ipmi_exporter configuration file.
	ConfigFile string `alloy:"config_file,attr,optional"`

	// IPMIConfig is the inline ipmi_exporter configuration.
	IPMIConfig util.RawYAML `alloy:"ipmi_config,attr,optional"`
}

// LocalConfig controls local IPMI collection.
type LocalConfig struct {
	// Enabled controls whether local IPMI collection is enabled.
	Enabled bool `alloy:"enabled,attr,optional"`

	// Module specifies which collector module to use for local collection.
	Module string `alloy:"module,attr,optional"`
}

// IPMITarget defines a target device to be monitored.
type IPMITarget struct {
	Name   string `alloy:"name,attr"`
	Target string `alloy:"address,attr"`
	Module string `alloy:"module,attr,optional"`

	// Authentication for remote IPMI
	User      string            `alloy:"user,attr,optional"`
	Password  alloytypes.Secret `alloy:"password,attr,optional"`
	Driver    string            `alloy:"driver,attr,optional"`    // LAN_2_0 or LAN
	Privilege string            `alloy:"privilege,attr,optional"` // user or admin
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if !a.Local.Enabled && len(a.Targets) == 0 {
		return fmt.Errorf("either local IPMI collection must be enabled or at least one remote target must be configured")
	}
	return nil
}

// Convert converts the component's Arguments to the integration's Config.
func (a *Arguments) Convert() *ipmi_exporter.Config {
	return &ipmi_exporter.Config{
		Local: ipmi_exporter.LocalConfig{
			Enabled: a.Local.Enabled,
			Module:  a.Local.Module,
		},
		Targets:    convertTargets(a.Targets),
		Timeout:    a.Timeout.Milliseconds(),
		ConfigFile: a.ConfigFile,
		IPMIConfig: a.IPMIConfig,
	}
}

func convertTargets(targets []IPMITarget) []ipmi_exporter.IPMITarget {
	result := make([]ipmi_exporter.IPMITarget, 0, len(targets))
	for _, t := range targets {
		result = append(result, ipmi_exporter.IPMITarget{
			Name:      t.Name,
			Target:    t.Target,
			Module:    t.Module,
			User:      t.User,
			Password:  config_util.Secret(t.Password),
			Driver:    t.Driver,
			Privilege: t.Privilege,
		})
	}
	return result
}
