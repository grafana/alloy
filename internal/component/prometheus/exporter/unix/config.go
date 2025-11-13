package unix

import (
	"time"

	node_integration "github.com/grafana/alloy/internal/static/integrations/node_exporter"
	"github.com/grafana/dskit/flagext"
)

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from YAML.
//
// Some defaults are populated from init functions in the github.com/grafana/alloy/internal/static/integrations/node_exporter package.
var DefaultArguments = Arguments{
	ProcFSPath:   node_integration.DefaultConfig.ProcFSPath,
	RootFSPath:   node_integration.DefaultConfig.RootFSPath,
	SysFSPath:    node_integration.DefaultConfig.SysFSPath,
	UdevDataPath: node_integration.DefaultConfig.UdevDataPath,
	Disk: DiskStatsConfig{
		DeviceExclude: node_integration.DefaultConfig.DiskStatsDeviceExclude,
	},
	EthTool: EthToolConfig{
		MetricsInclude: ".*",
	},
	Filesystem: FilesystemConfig{
		MountTimeout:       5 * time.Second,
		MountPointsExclude: node_integration.DefaultConfig.FilesystemMountPointsExclude,
		FSTypesExclude:     node_integration.DefaultConfig.FilesystemFSTypesExclude,
	},
	NTP: NTPConfig{
		IPTTL:                1,
		LocalOffsetTolerance: time.Millisecond,
		MaxDistance:          time.Microsecond * 3466080,
		ProtocolVersion:      4,
		Server:               "127.0.0.1",
	},
	Netclass: NetclassConfig{
		IgnoredDevices: "^$",
	},
	Netstat: NetstatConfig{
		Fields: node_integration.DefaultConfig.NetstatFields,
	},
	Powersupply: PowersupplyConfig{
		IgnoredSupplies: "^$",
	},
	Runit: RunitConfig{
		ServiceDir: "/etc/service",
	},
	Supervisord: SupervisordConfig{
		URL: node_integration.DefaultConfig.SupervisordURL,
	},
	Systemd: SystemdConfig{
		UnitExclude: node_integration.DefaultConfig.SystemdUnitExclude,
		UnitInclude: ".+",
	},
	Tapestats: TapestatsConfig{
		IgnoredDevices: "^$",
	},
	VMStat: VMStatConfig{
		Fields: node_integration.DefaultConfig.VMStatFields,
	},
}

// Arguments is used for controlling for this exporter.
type Arguments struct {
	IncludeExporterMetrics bool   `alloy:"include_exporter_metrics,attr,optional"`
	ProcFSPath             string `alloy:"procfs_path,attr,optional"`
	SysFSPath              string `alloy:"sysfs_path,attr,optional"`
	RootFSPath             string `alloy:"rootfs_path,attr,optional"`
	UdevDataPath           string `alloy:"udev_data_path,attr,optional"`

	// Collectors to mark as enabled
	EnableCollectors flagext.StringSlice `alloy:"enable_collectors,attr,optional"`

	// Collectors to mark as disabled
	DisableCollectors flagext.StringSlice `alloy:"disable_collectors,attr,optional"`

	// Overrides the default set of enabled collectors with the collectors
	// listed.
	SetCollectors flagext.StringSlice `alloy:"set_collectors,attr,optional"`

	// Collector-specific config options
	Arp         ArpConfig         `alloy:"arp,block,optional"`
	BCache      BCacheConfig      `alloy:"bcache,block,optional"`
	CPU         CPUConfig         `alloy:"cpu,block,optional"`
	Disk        DiskStatsConfig   `alloy:"disk,block,optional"`
	EthTool     EthToolConfig     `alloy:"ethtool,block,optional"`
	Filesystem  FilesystemConfig  `alloy:"filesystem,block,optional"`
	HwMon       HwMonConfig       `alloy:"hwmon,block,optional"`
	IPVS        IPVSConfig        `alloy:"ipvs,block,optional"`
	NTP         NTPConfig         `alloy:"ntp,block,optional"`
	Netclass    NetclassConfig    `alloy:"netclass,block,optional"`
	Netdev      NetdevConfig      `alloy:"netdev,block,optional"`
	Netstat     NetstatConfig     `alloy:"netstat,block,optional"`
	Perf        PerfConfig        `alloy:"perf,block,optional"`
	Powersupply PowersupplyConfig `alloy:"powersupply,block,optional"`
	Runit       RunitConfig       `alloy:"runit,block,optional"`
	Supervisord SupervisordConfig `alloy:"supervisord,block,optional"`
	Sysctl      SysctlConfig      `alloy:"sysctl,block,optional"`
	Systemd     SystemdConfig     `alloy:"systemd,block,optional"`
	Tapestats   TapestatsConfig   `alloy:"tapestats,block,optional"`
	Textfile    TextfileConfig    `alloy:"textfile,block,optional"`
	VMStat      VMStatConfig      `alloy:"vmstat,block,optional"`
}

// Convert gives a config suitable for use with github.com/grafana/alloy/internal/static/integrations/node_exporter.
func (a *Arguments) Convert() *node_integration.Config {
	return &node_integration.Config{
		IncludeExporterMetrics:           a.IncludeExporterMetrics,
		ProcFSPath:                       a.ProcFSPath,
		SysFSPath:                        a.SysFSPath,
		RootFSPath:                       a.RootFSPath,
		UdevDataPath:                     a.UdevDataPath,
		EnableCollectors:                 a.EnableCollectors,
		DisableCollectors:                a.DisableCollectors,
		SetCollectors:                    a.SetCollectors,
		ArpDeviceExclude:                 a.Arp.DeviceExclude,
		ArpDeviceInclude:                 a.Arp.DeviceInclude,
		ArpNetlink:                       a.Arp.Netlink,
		BcachePriorityStats:              a.BCache.PriorityStats,
		CPUBugsInclude:                   a.CPU.BugsInclude,
		CPUEnableCPUGuest:                a.CPU.EnableCPUGuest,
		CPUEnableCPUInfo:                 a.CPU.EnableCPUInfo,
		CPUFlagsInclude:                  a.CPU.FlagsInclude,
		DiskStatsDeviceExclude:           a.Disk.DeviceExclude,
		DiskStatsDeviceInclude:           a.Disk.DeviceInclude,
		EthtoolDeviceExclude:             a.EthTool.DeviceExclude,
		EthtoolDeviceInclude:             a.EthTool.DeviceInclude,
		EthtoolMetricsInclude:            a.EthTool.MetricsInclude,
		FilesystemFSTypesExclude:         a.Filesystem.FSTypesExclude,
		FilesystemMountPointsExclude:     a.Filesystem.MountPointsExclude,
		FilesystemMountTimeout:           a.Filesystem.MountTimeout,
		HwMonChipInclude:                 a.HwMon.ChipInclude,
		HwMonChipExclude:                 a.HwMon.ChipExclude,
		IPVSBackendLabels:                a.IPVS.BackendLabels,
		NTPIPTTL:                         a.NTP.IPTTL,
		NTPLocalOffsetTolerance:          a.NTP.LocalOffsetTolerance,
		NTPMaxDistance:                   a.NTP.MaxDistance,
		NTPProtocolVersion:               a.NTP.ProtocolVersion,
		NTPServer:                        a.NTP.Server,
		NTPServerIsLocal:                 a.NTP.ServerIsLocal,
		NetclassIgnoreInvalidSpeedDevice: a.Netclass.IgnoreInvalidSpeedDevice,
		NetclassIgnoredDevices:           a.Netclass.IgnoredDevices,
		NetdevAddressInfo:                a.Netdev.AddressInfo,
		NetdevDeviceExclude:              a.Netdev.DeviceExclude,
		NetdevDeviceInclude:              a.Netdev.DeviceInclude,
		NetstatFields:                    a.Netstat.Fields,
		PerfCPUS:                         a.Perf.CPUS,
		PerfTracepoint:                   a.Perf.Tracepoint,
		PerfDisableHardwareProfilers:     a.Perf.DisableHardwareProfilers,
		PerfHardwareProfilers:            a.Perf.HardwareProfilers,
		PerfDisableSoftwareProfilers:     a.Perf.DisableSoftwareProfilers,
		PerfSoftwareProfilers:            a.Perf.SoftwareProfilers,
		PerfDisableCacheProfilers:        a.Perf.DisableCacheProfilers,
		PerfCacheProfilers:               a.Perf.CacheProfilers,
		PowersupplyIgnoredSupplies:       a.Powersupply.IgnoredSupplies,
		RunitServiceDir:                  a.Runit.ServiceDir,
		SupervisordURL:                   a.Supervisord.URL,
		SysctlInclude:                    a.Sysctl.Include,
		SysctlIncludeInfo:                a.Sysctl.IncludeInfo,
		SystemdEnableRestartsMetrics:     a.Systemd.EnableRestartsMetrics,
		SystemdEnableStartTimeMetrics:    a.Systemd.EnableStartTimeMetrics,
		SystemdEnableTaskMetrics:         a.Systemd.EnableTaskMetrics,
		SystemdUnitExclude:               a.Systemd.UnitExclude,
		SystemdUnitInclude:               a.Systemd.UnitInclude,
		TapestatsIgnoredDevices:          a.Tapestats.IgnoredDevices,
		TextfileDirectory:                a.Textfile.Directory,
		VMStatFields:                     a.VMStat.Fields,
	}
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// PowersupplyConfig contains config specific to the powersupply collector.
type PowersupplyConfig struct {
	IgnoredSupplies string `alloy:"ignored_supplies,attr,optional"`
}

// RunitConfig contains config specific to the runit collector.
type RunitConfig struct {
	ServiceDir string `alloy:"service_dir,attr,optional"`
}

// SupervisordConfig contains config specific to the supervisord collector.
type SupervisordConfig struct {
	URL string `alloy:"url,attr,optional"`
}

// TapestatsConfig contains config specific to the tapestats collector.
type TapestatsConfig struct {
	IgnoredDevices string `alloy:"ignored_devices,attr,optional"`
}

// TextfileConfig contains config specific to the textfile collector.
type TextfileConfig struct {
	Directory string `alloy:"directory,attr,optional"`
}

// VMStatConfig contains config specific to the vmstat collector.
type VMStatConfig struct {
	Fields string `alloy:"fields,attr,optional"`
}

// NetclassConfig contains config specific to the netclass collector.
type NetclassConfig struct {
	IgnoreInvalidSpeedDevice bool   `alloy:"ignore_invalid_speed_device,attr,optional"`
	IgnoredDevices           string `alloy:"ignored_devices,attr,optional"`
}

// NetdevConfig contains config specific to the netdev collector.
type NetdevConfig struct {
	AddressInfo   bool   `alloy:"address_info,attr,optional"`
	DeviceExclude string `alloy:"device_exclude,attr,optional"`
	DeviceInclude string `alloy:"device_include,attr,optional"`
}

// NetstatConfig contains config specific to the netstat collector.
type NetstatConfig struct {
	Fields string `alloy:"fields,attr,optional"`
}

// PerfConfig contains config specific to the perf collector.
type PerfConfig struct {
	CPUS       string              `alloy:"cpus,attr,optional"`
	Tracepoint flagext.StringSlice `alloy:"tracepoint,attr,optional"`

	DisableHardwareProfilers bool `alloy:"disable_hardware_profilers,attr,optional"`
	DisableSoftwareProfilers bool `alloy:"disable_software_profilers,attr,optional"`
	DisableCacheProfilers    bool `alloy:"disable_cache_profilers,attr,optional"`

	HardwareProfilers flagext.StringSlice `alloy:"hardware_profilers,attr,optional"`
	SoftwareProfilers flagext.StringSlice `alloy:"software_profilers,attr,optional"`
	CacheProfilers    flagext.StringSlice `alloy:"cache_profilers,attr,optional"`
}

// EthToolConfig contains config specific to the ethtool collector.
type EthToolConfig struct {
	DeviceExclude  string `alloy:"device_exclude,attr,optional"`
	DeviceInclude  string `alloy:"device_include,attr,optional"`
	MetricsInclude string `alloy:"metrics_include,attr,optional"`
}

// HwMonConfig contains config specific to the hwmon collector.
type HwMonConfig struct {
	ChipExclude string `alloy:"chip_exclude,attr,optional"`
	ChipInclude string `alloy:"chip_include,attr,optional"`
}

// FilesystemConfig contains config specific to the filesystem collector.
type FilesystemConfig struct {
	FSTypesExclude     string        `alloy:"fs_types_exclude,attr,optional"`
	MountPointsExclude string        `alloy:"mount_points_exclude,attr,optional"`
	MountTimeout       time.Duration `alloy:"mount_timeout,attr,optional"`
}

// IPVSConfig contains config specific to the ipvs collector.
type IPVSConfig struct {
	BackendLabels []string `alloy:"backend_labels,attr,optional"`
}

// ArpConfig contains config specific to the arp collector.
type ArpConfig struct {
	DeviceExclude string `alloy:"device_exclude,attr,optional"`
	DeviceInclude string `alloy:"device_include,attr,optional"`
	Netlink       bool   `alloy:"netlink,attr,optional"`
}

// BCacheConfig contains config specific to the bcache collector.
type BCacheConfig struct {
	PriorityStats bool `alloy:"priority_stats,attr,optional"`
}

// CPUConfig contains config specific to the cpu collector.
type CPUConfig struct {
	BugsInclude    string `alloy:"bugs_include,attr,optional"`
	EnableCPUGuest bool   `alloy:"guest,attr,optional"`
	EnableCPUInfo  bool   `alloy:"info,attr,optional"`
	FlagsInclude   string `alloy:"flags_include,attr,optional"`
}

// DiskStatsConfig contains config specific to the diskstats collector.
type DiskStatsConfig struct {
	DeviceExclude string `alloy:"device_exclude,attr,optional"`
	DeviceInclude string `alloy:"device_include,attr,optional"`
}

// NTPConfig contains config specific to the ntp collector.
type NTPConfig struct {
	IPTTL                int           `alloy:"ip_ttl,attr,optional"`
	LocalOffsetTolerance time.Duration `alloy:"local_offset_tolerance,attr,optional"`
	MaxDistance          time.Duration `alloy:"max_distance,attr,optional"`
	ProtocolVersion      int           `alloy:"protocol_version,attr,optional"`
	Server               string        `alloy:"server,attr,optional"`
	ServerIsLocal        bool          `alloy:"server_is_local,attr,optional"`
}

// SystemdConfig contains config specific to the systemd collector.
type SystemdConfig struct {
	EnableRestartsMetrics  bool   `alloy:"enable_restarts,attr,optional"`
	EnableStartTimeMetrics bool   `alloy:"start_time,attr,optional"`
	EnableTaskMetrics      bool   `alloy:"task_metrics,attr,optional"`
	UnitExclude            string `alloy:"unit_exclude,attr,optional"`
	UnitInclude            string `alloy:"unit_include,attr,optional"`
}

// SysctlConfig contains config specific to the sysctl collector.
type SysctlConfig struct {
	Include     []string `alloy:"include,attr,optional"`
	IncludeInfo []string `alloy:"include_info,attr,optional"`
}
