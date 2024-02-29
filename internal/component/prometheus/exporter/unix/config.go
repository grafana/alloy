package unix

import (
	"time"

	node_integration "github.com/grafana/agent/internal/static/integrations/node_exporter"
	"github.com/grafana/dskit/flagext"
)

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from YAML.
//
// Some defaults are populated from init functions in the github.com/grafana/agent/internal/static/integrations/node_exporter package.
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
	IncludeExporterMetrics bool   `river:"include_exporter_metrics,attr,optional"`
	ProcFSPath             string `river:"procfs_path,attr,optional"`
	SysFSPath              string `river:"sysfs_path,attr,optional"`
	RootFSPath             string `river:"rootfs_path,attr,optional"`
	UdevDataPath           string `river:"udev_data_path,attr,optional"`

	// Collectors to mark as enabled
	EnableCollectors flagext.StringSlice `river:"enable_collectors,attr,optional"`

	// Collectors to mark as disabled
	DisableCollectors flagext.StringSlice `river:"disable_collectors,attr,optional"`

	// Overrides the default set of enabled collectors with the collectors
	// listed.
	SetCollectors flagext.StringSlice `river:"set_collectors,attr,optional"`

	// Collector-specific config options
	BCache      BCacheConfig      `river:"bcache,block,optional"`
	CPU         CPUConfig         `river:"cpu,block,optional"`
	Disk        DiskStatsConfig   `river:"disk,block,optional"`
	EthTool     EthToolConfig     `river:"ethtool,block,optional"`
	Filesystem  FilesystemConfig  `river:"filesystem,block,optional"`
	IPVS        IPVSConfig        `river:"ipvs,block,optional"`
	NTP         NTPConfig         `river:"ntp,block,optional"`
	Netclass    NetclassConfig    `river:"netclass,block,optional"`
	Netdev      NetdevConfig      `river:"netdev,block,optional"`
	Netstat     NetstatConfig     `river:"netstat,block,optional"`
	Perf        PerfConfig        `river:"perf,block,optional"`
	Powersupply PowersupplyConfig `river:"powersupply,block,optional"`
	Runit       RunitConfig       `river:"runit,block,optional"`
	Supervisord SupervisordConfig `river:"supervisord,block,optional"`
	Sysctl      SysctlConfig      `river:"sysctl,block,optional"`
	Systemd     SystemdConfig     `river:"systemd,block,optional"`
	Tapestats   TapestatsConfig   `river:"tapestats,block,optional"`
	Textfile    TextfileConfig    `river:"textfile,block,optional"`
	VMStat      VMStatConfig      `river:"vmstat,block,optional"`
}

// Convert gives a config suitable for use with github.com/grafana/agent/internal/static/integrations/node_exporter.
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

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// PowersupplyConfig contains config specific to the powersupply collector.
type PowersupplyConfig struct {
	IgnoredSupplies string `river:"ignored_supplies,attr,optional"`
}

// RunitConfig contains config specific to the runit collector.
type RunitConfig struct {
	ServiceDir string `river:"service_dir,attr,optional"`
}

// SupervisordConfig contains config specific to the supervisord collector.
type SupervisordConfig struct {
	URL string `river:"url,attr,optional"`
}

// TapestatsConfig contains config specific to the tapestats collector.
type TapestatsConfig struct {
	IgnoredDevices string `river:"ignored_devices,attr,optional"`
}

// TextfileConfig contains config specific to the textfile collector.
type TextfileConfig struct {
	Directory string `river:"directory,attr,optional"`
}

// VMStatConfig contains config specific to the vmstat collector.
type VMStatConfig struct {
	Fields string `river:"fields,attr,optional"`
}

// NetclassConfig contains config specific to the netclass collector.
type NetclassConfig struct {
	IgnoreInvalidSpeedDevice bool   `river:"ignore_invalid_speed_device,attr,optional"`
	IgnoredDevices           string `river:"ignored_devices,attr,optional"`
}

// NetdevConfig contains config specific to the netdev collector.
type NetdevConfig struct {
	AddressInfo   bool   `river:"address_info,attr,optional"`
	DeviceExclude string `river:"device_exclude,attr,optional"`
	DeviceInclude string `river:"device_include,attr,optional"`
}

// NetstatConfig contains config specific to the netstat collector.
type NetstatConfig struct {
	Fields string `river:"fields,attr,optional"`
}

// PerfConfig contains config specific to the perf collector.
type PerfConfig struct {
	CPUS       string              `river:"cpus,attr,optional"`
	Tracepoint flagext.StringSlice `river:"tracepoint,attr,optional"`

	DisableHardwareProfilers bool `river:"disable_hardware_profilers,attr,optional"`
	DisableSoftwareProfilers bool `river:"disable_software_profilers,attr,optional"`
	DisableCacheProfilers    bool `river:"disable_cache_profilers,attr,optional"`

	HardwareProfilers flagext.StringSlice `river:"hardware_profilers,attr,optional"`
	SoftwareProfilers flagext.StringSlice `river:"software_profilers,attr,optional"`
	CacheProfilers    flagext.StringSlice `river:"cache_profilers,attr,optional"`
}

// EthToolConfig contains config specific to the ethtool collector.
type EthToolConfig struct {
	DeviceExclude  string `river:"device_exclude,attr,optional"`
	DeviceInclude  string `river:"device_include,attr,optional"`
	MetricsInclude string `river:"metrics_include,attr,optional"`
}

// FilesystemConfig contains config specific to the filesystem collector.
type FilesystemConfig struct {
	FSTypesExclude     string        `river:"fs_types_exclude,attr,optional"`
	MountPointsExclude string        `river:"mount_points_exclude,attr,optional"`
	MountTimeout       time.Duration `river:"mount_timeout,attr,optional"`
}

// IPVSConfig contains config specific to the ipvs collector.
type IPVSConfig struct {
	BackendLabels []string `river:"backend_labels,attr,optional"`
}

// BCacheConfig contains config specific to the bcache collector.
type BCacheConfig struct {
	PriorityStats bool `river:"priority_stats,attr,optional"`
}

// CPUConfig contains config specific to the cpu collector.
type CPUConfig struct {
	BugsInclude    string `river:"bugs_include,attr,optional"`
	EnableCPUGuest bool   `river:"guest,attr,optional"`
	EnableCPUInfo  bool   `river:"info,attr,optional"`
	FlagsInclude   string `river:"flags_include,attr,optional"`
}

// DiskStatsConfig contains config specific to the diskstats collector.
type DiskStatsConfig struct {
	DeviceExclude string `river:"device_exclude,attr,optional"`
	DeviceInclude string `river:"device_include,attr,optional"`
}

// NTPConfig contains config specific to the ntp collector.
type NTPConfig struct {
	IPTTL                int           `river:"ip_ttl,attr,optional"`
	LocalOffsetTolerance time.Duration `river:"local_offset_tolerance,attr,optional"`
	MaxDistance          time.Duration `river:"max_distance,attr,optional"`
	ProtocolVersion      int           `river:"protocol_version,attr,optional"`
	Server               string        `river:"server,attr,optional"`
	ServerIsLocal        bool          `river:"server_is_local,attr,optional"`
}

// SystemdConfig contains config specific to the systemd collector.
type SystemdConfig struct {
	EnableRestartsMetrics  bool   `river:"enable_restarts,attr,optional"`
	EnableStartTimeMetrics bool   `river:"start_time,attr,optional"`
	EnableTaskMetrics      bool   `river:"task_metrics,attr,optional"`
	UnitExclude            string `river:"unit_exclude,attr,optional"`
	UnitInclude            string `river:"unit_include,attr,optional"`
}

// SysctlConfig contains config specific to the sysctl collector.
type SysctlConfig struct {
	Include     []string `river:"include,attr,optional"`
	IncludeInfo []string `river:"include_info,attr,optional"`
}
