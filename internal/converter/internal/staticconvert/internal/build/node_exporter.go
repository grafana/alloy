package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/unix"
	"github.com/grafana/alloy/internal/static/integrations/node_exporter"
)

func (b *ConfigBuilder) appendNodeExporter(config *node_exporter.Config, instanceKey *string) discovery.Exports {
	args := toNodeExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "unix")
}

func toNodeExporter(config *node_exporter.Config) *unix.Arguments {
	return &unix.Arguments{
		IncludeExporterMetrics: config.IncludeExporterMetrics,
		ProcFSPath:             config.ProcFSPath,
		SysFSPath:              config.SysFSPath,
		RootFSPath:             config.RootFSPath,
		UdevDataPath:           config.UdevDataPath,
		EnableCollectors:       config.EnableCollectors,
		DisableCollectors:      config.DisableCollectors,
		SetCollectors:          config.SetCollectors,
		BCache: unix.BCacheConfig{
			PriorityStats: config.BcachePriorityStats,
		},
		CPU: unix.CPUConfig{
			BugsInclude:    config.CPUBugsInclude,
			EnableCPUGuest: config.CPUEnableCPUGuest,
			EnableCPUInfo:  config.CPUEnableCPUInfo,
			FlagsInclude:   config.CPUFlagsInclude,
		},
		Disk: unix.DiskStatsConfig{
			DeviceExclude: config.DiskStatsDeviceExclude,
			DeviceInclude: config.DiskStatsDeviceInclude,
		},
		EthTool: unix.EthToolConfig{
			DeviceExclude:  config.EthtoolDeviceExclude,
			DeviceInclude:  config.EthtoolDeviceInclude,
			MetricsInclude: config.EthtoolMetricsInclude,
		},
		Filesystem: unix.FilesystemConfig{
			FSTypesExclude:     config.FilesystemFSTypesExclude,
			MountPointsExclude: config.FilesystemMountPointsExclude,
			MountTimeout:       config.FilesystemMountTimeout,
		},
		HwMon: unix.HwMonConfig{
			ChipExclude: config.HwMonChipExclude,
			ChipInclude: config.HwMonChipInclude,
		},
		IPVS: unix.IPVSConfig{
			BackendLabels: config.IPVSBackendLabels,
		},
		NTP: unix.NTPConfig{
			IPTTL:                config.NTPIPTTL,
			LocalOffsetTolerance: config.NTPLocalOffsetTolerance,
			MaxDistance:          config.NTPMaxDistance,
			ProtocolVersion:      config.NTPProtocolVersion,
			Server:               config.NTPServer,
			ServerIsLocal:        config.NTPServerIsLocal,
		},
		Netclass: unix.NetclassConfig{
			IgnoreInvalidSpeedDevice: config.NetclassIgnoreInvalidSpeedDevice,
			IgnoredDevices:           config.NetclassIgnoredDevices,
		},
		Netdev: unix.NetdevConfig{
			AddressInfo:   config.NetdevAddressInfo,
			DeviceExclude: config.NetdevDeviceExclude,
			DeviceInclude: config.NetdevDeviceInclude,
		},
		Netstat: unix.NetstatConfig{
			Fields: config.NetstatFields,
		},
		Perf: unix.PerfConfig{
			CPUS:                     config.PerfCPUS,
			Tracepoint:               config.PerfTracepoint,
			DisableHardwareProfilers: config.PerfDisableHardwareProfilers,
			DisableSoftwareProfilers: config.PerfDisableSoftwareProfilers,
			DisableCacheProfilers:    config.PerfDisableCacheProfilers,
			HardwareProfilers:        config.PerfHardwareProfilers,
			SoftwareProfilers:        config.PerfSoftwareProfilers,
			CacheProfilers:           config.PerfCacheProfilers,
		},
		Powersupply: unix.PowersupplyConfig{
			IgnoredSupplies: config.PowersupplyIgnoredSupplies,
		},
		Runit: unix.RunitConfig{
			ServiceDir: config.RunitServiceDir,
		},
		Supervisord: unix.SupervisordConfig{
			URL: config.SupervisordURL,
		},
		Sysctl: unix.SysctlConfig{
			Include:     config.SysctlInclude,
			IncludeInfo: config.SysctlIncludeInfo,
		},
		Systemd: unix.SystemdConfig{
			EnableRestartsMetrics:  config.SystemdEnableRestartsMetrics,
			EnableStartTimeMetrics: config.SystemdEnableStartTimeMetrics,
			EnableTaskMetrics:      config.SystemdEnableTaskMetrics,
			UnitExclude:            config.SystemdUnitExclude,
			UnitInclude:            config.SystemdUnitInclude,
		},
		Tapestats: unix.TapestatsConfig{
			IgnoredDevices: config.TapestatsIgnoredDevices,
		},
		Textfile: unix.TextfileConfig{
			Directory: config.TextfileDirectory,
		},
		VMStat: unix.VMStatConfig{
			Fields: config.VMStatFields,
		},
	}
}
