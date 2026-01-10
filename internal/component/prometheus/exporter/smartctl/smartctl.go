// Package smartctl provides an Alloy component for the smartctl_exporter.
package smartctl

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/common"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/smartctl_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.smartctl",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "smartctl"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	common.WarningIfUsedInCluster(opts)
	a := args.(Arguments)
	defaultInstanceKey := common.HostNameInstanceKey()
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the smartctl exporter.
var DefaultArguments = Arguments{
	SmartctlPath:   "/usr/sbin/smartctl",
	ScanInterval:   60 * time.Second,
	RescanInterval: 10 * time.Minute,
	PowermodeCheck: "standby",
}

// Arguments controls the prometheus.exporter.smartctl component.
type Arguments struct {
	// SmartctlPath is the path to the smartctl binary.
	// The smartctl binary is part of the smartmontools package and must be
	// installed on the system. Version 7.0 or later is required for JSON support.
	SmartctlPath string `alloy:"smartctl_path,attr,optional"`

	// ScanInterval is how often to poll smartctl for device data.
	// Smartctl queries can be slow, especially with many drives, so the default
	// is 60 seconds to prevent system overload.
	ScanInterval time.Duration `alloy:"scan_interval,attr,optional"`

	// RescanInterval is how often to rescan for new/removed devices.
	// Set to 0 to disable automatic rescanning.
	// Only applies when devices are discovered automatically (devices not specified).
	RescanInterval time.Duration `alloy:"rescan_interval,attr,optional"`

	// Devices is a list of specific devices to monitor.
	// When specified, automatic device discovery is disabled.
	// Examples: ["/dev/sda", "/dev/nvme0n1"]
	Devices []string `alloy:"devices,attr,optional"`

	// DeviceExclude is a regex pattern to exclude devices from automatic scanning.
	// Mutually exclusive with DeviceInclude.
	// Example: "^(ram|loop|fd)\\d+$" excludes RAM disks, loop devices, and floppy drives.
	DeviceExclude string `alloy:"device_exclude,attr,optional"`

	// DeviceInclude is a regex pattern to include only matching devices in automatic scanning.
	// Mutually exclusive with DeviceExclude.
	// Example: "^(sd|nvme)" includes only SATA and NVMe devices.
	DeviceInclude string `alloy:"device_include,attr,optional"`

	// ScanDeviceTypes controls the device types to scan.
	// Common values: ["sat"] (SATA), ["scsi"] (SAS/SCSI), ["nvme"] (NVMe), ["auto"] (auto-detect)
	// Default: auto-detect
	ScanDeviceTypes []string `alloy:"scan_device_types,attr,optional"`

	// PowermodeCheck determines when to skip checking devices based on power mode.
	// This prevents waking up drives that are in low-power states.
	// Options: "never" (always check), "sleep", "standby", "idle"
	// Default: "standby" (skip devices in standby mode or deeper sleep states)
	PowermodeCheck string `alloy:"powermode_check,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.DeviceExclude != "" && a.DeviceInclude != "" {
		return fmt.Errorf("device_exclude and device_include are mutually exclusive")
	}

	validPowermodes := map[string]bool{
		"never":   true,
		"sleep":   true,
		"standby": true,
		"idle":    true,
	}
	if a.PowermodeCheck != "" && !validPowermodes[a.PowermodeCheck] {
		return fmt.Errorf("invalid powermode_check: %s (must be never, sleep, standby, or idle)", a.PowermodeCheck)
	}

	return nil
}

// Convert converts Arguments to the integration's Config type.
func (a *Arguments) Convert() *smartctl_exporter.Config {
	return &smartctl_exporter.Config{
		SmartctlPath:    a.SmartctlPath,
		ScanInterval:    a.ScanInterval,
		RescanInterval:  a.RescanInterval,
		Devices:         a.Devices,
		DeviceExclude:   a.DeviceExclude,
		DeviceInclude:   a.DeviceInclude,
		ScanDeviceTypes: a.ScanDeviceTypes,
		PowermodeCheck:  a.PowermodeCheck,
	}
}
