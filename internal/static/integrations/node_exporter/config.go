package node_exporter

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/node_exporter/collector"
	"github.com/prometheus/procfs"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

var (
	// DefaultConfig holds non-zero default options for the Config when it is
	// unmarshaled from YAML.
	//
	// DefaultConfig's defaults are populated from init functions in this package.
	// See the init function here and in node_exporter_linux.go.
	DefaultConfig = Config{
		ProcFSPath:   procfs.DefaultMountPoint,
		RootFSPath:   "/",
		UdevDataPath: "/run/udev/data",

		DiskStatsDeviceExclude: "^(ram|loop|fd|(h|s|v|xv)d[a-z]|nvme\\d+n\\d+p)\\d+$",

		EthtoolMetricsInclude: ".*",

		FilesystemMountTimeout: 5 * time.Second,

		NTPIPTTL:                1,
		NTPLocalOffsetTolerance: time.Millisecond,
		NTPMaxDistance:          time.Microsecond * 3466080,
		NTPProtocolVersion:      4,
		NTPServer:               "127.0.0.1",

		NetclassIgnoredDevices: "^$",
		NetstatFields:          "^(.*_(InErrors|InErrs)|Ip_Forwarding|Ip(6|Ext)_(InOctets|OutOctets)|Icmp6?_(InMsgs|OutMsgs)|TcpExt_(Listen.*|Syncookies.*|TCPSynRetrans|TCPTimeouts)|Tcp_(ActiveOpens|InSegs|OutSegs|OutRsts|PassiveOpens|RetransSegs|CurrEstab)|Udp6?_(InDatagrams|OutDatagrams|NoPorts|RcvbufErrors|SndbufErrors))$",

		PowersupplyIgnoredSupplies: "^$",

		RunitServiceDir: "/etc/service",

		SupervisordURL: "http://localhost:9001/RPC2",

		SystemdUnitExclude: ".+\\.(automount|device|mount|scope|slice)",
		SystemdUnitInclude: ".+",

		TapestatsIgnoredDevices: "^$",

		VMStatFields: "^(oom_kill|pgpg|pswp|pg.*fault).*",
	}
)

func init() {
	// The default values for the filesystem collector are to ignore everything,
	// but some platforms have specific defaults. We'll fill these in below at
	// initialization time, but the values can still be overridden via the config
	// file.
	switch runtime.GOOS {
	case "linux":
		DefaultConfig.FilesystemMountPointsExclude = "^/(dev|proc|run/credentials/.+|sys|var/lib/docker/.+)($|/)"
		DefaultConfig.FilesystemFSTypesExclude = "^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs)$"
	case "darwin":
		DefaultConfig.FilesystemMountPointsExclude = "^/(dev)($|/)"
		DefaultConfig.FilesystemFSTypesExclude = "^(autofs|devfs)$"
	case "freebsd", "netbsd", "openbsd":
		DefaultConfig.FilesystemMountPointsExclude = "^/(dev)($|/)"
		DefaultConfig.FilesystemFSTypesExclude = "^devfs$"
	}

	if url := os.Getenv("SUPERVISORD_URL"); url != "" {
		DefaultConfig.SupervisordURL = url
	}
}

// Config controls the node_exporter integration.
type Config struct {
	IncludeExporterMetrics bool `yaml:"include_exporter_metrics,omitempty"`

	ProcFSPath   string `yaml:"procfs_path,omitempty"`
	SysFSPath    string `yaml:"sysfs_path,omitempty"`
	RootFSPath   string `yaml:"rootfs_path,omitempty"`
	UdevDataPath string `yaml:"udev_data_path,omitempty"`

	// Collectors to mark as enabled
	EnableCollectors flagext.StringSlice `yaml:"enable_collectors,omitempty"`

	// Collectors to mark as disabled
	DisableCollectors flagext.StringSlice `yaml:"disable_collectors,omitempty"`

	// Overrides the default set of enabled collectors with the collectors
	// listed.
	SetCollectors flagext.StringSlice `yaml:"set_collectors,omitempty"`

	// Collector-specific config options
	BcachePriorityStats              bool                `yaml:"enable_bcache_priority_stats,omitempty"`
	CPUBugsInclude                   string              `yaml:"cpu_bugs_include,omitempty"`
	CPUEnableCPUGuest                bool                `yaml:"enable_cpu_guest_seconds_metric,omitempty"`
	CPUEnableCPUInfo                 bool                `yaml:"enable_cpu_info_metric,omitempty"`
	CPUFlagsInclude                  string              `yaml:"cpu_flags_include,omitempty"`
	DiskStatsDeviceExclude           string              `yaml:"diskstats_device_exclude,omitempty"`
	DiskStatsDeviceInclude           string              `yaml:"diskstats_device_include,omitempty"`
	EthtoolDeviceExclude             string              `yaml:"ethtool_device_exclude,omitempty"`
	EthtoolDeviceInclude             string              `yaml:"ethtool_device_include,omitempty"`
	EthtoolMetricsInclude            string              `yaml:"ethtool_metrics_include,omitempty"`
	FilesystemFSTypesExclude         string              `yaml:"filesystem_fs_types_exclude,omitempty"`
	FilesystemMountPointsExclude     string              `yaml:"filesystem_mount_points_exclude,omitempty"`
	FilesystemMountTimeout           time.Duration       `yaml:"filesystem_mount_timeout,omitempty"`
	HwMonChipInclude                 string              `yaml:"hwmon_chip_include,omitempty"`
	HwMonChipExclude                 string              `yaml:"hwmon_chip_exclude,omitempty"`
	IPVSBackendLabels                []string            `yaml:"ipvs_backend_labels,omitempty"`
	NTPIPTTL                         int                 `yaml:"ntp_ip_ttl,omitempty"`
	NTPLocalOffsetTolerance          time.Duration       `yaml:"ntp_local_offset_tolerance,omitempty"`
	NTPMaxDistance                   time.Duration       `yaml:"ntp_max_distance,omitempty"`
	NTPProtocolVersion               int                 `yaml:"ntp_protocol_version,omitempty"`
	NTPServer                        string              `yaml:"ntp_server,omitempty"`
	NTPServerIsLocal                 bool                `yaml:"ntp_server_is_local,omitempty"`
	NetclassIgnoreInvalidSpeedDevice bool                `yaml:"netclass_ignore_invalid_speed_device,omitempty"`
	NetclassIgnoredDevices           string              `yaml:"netclass_ignored_devices,omitempty"`
	NetdevAddressInfo                bool                `yaml:"netdev_address_info,omitempty"`
	NetdevDeviceExclude              string              `yaml:"netdev_device_exclude,omitempty"`
	NetdevDeviceInclude              string              `yaml:"netdev_device_include,omitempty"`
	NetstatFields                    string              `yaml:"netstat_fields,omitempty"`
	PerfCPUS                         string              `yaml:"perf_cpus,omitempty"`
	PerfTracepoint                   flagext.StringSlice `yaml:"perf_tracepoint,omitempty"`
	PerfDisableHardwareProfilers     bool                `yaml:"perf_disable_hardware_profilers,omitempty"`
	PerfDisableSoftwareProfilers     bool                `yaml:"perf_disable_software_profilers,omitempty"`
	PerfDisableCacheProfilers        bool                `yaml:"perf_disable_cache_profilers,omitempty"`
	PerfHardwareProfilers            flagext.StringSlice `yaml:"perf_hardware_profilers,omitempty"`
	PerfSoftwareProfilers            flagext.StringSlice `yaml:"perf_software_profilers,omitempty"`
	PerfCacheProfilers               flagext.StringSlice `yaml:"perf_cache_profilers,omitempty"`
	PowersupplyIgnoredSupplies       string              `yaml:"powersupply_ignored_supplies,omitempty"`
	RunitServiceDir                  string              `yaml:"runit_service_dir,omitempty"`
	SupervisordURL                   string              `yaml:"supervisord_url,omitempty"`
	SysctlInclude                    flagext.StringSlice `yaml:"sysctl_include,omitempty"`
	SysctlIncludeInfo                flagext.StringSlice `yaml:"sysctl_include_info,omitempty"`
	SystemdEnableRestartsMetrics     bool                `yaml:"systemd_enable_restarts_metrics,omitempty"`
	SystemdEnableStartTimeMetrics    bool                `yaml:"systemd_enable_start_time_metrics,omitempty"`
	SystemdEnableTaskMetrics         bool                `yaml:"systemd_enable_task_metrics,omitempty"`
	SystemdUnitExclude               string              `yaml:"systemd_unit_exclude,omitempty"`
	SystemdUnitInclude               string              `yaml:"systemd_unit_include,omitempty"`
	TapestatsIgnoredDevices          string              `yaml:"tapestats_ignored_devices,omitempty"`
	TextfileDirectory                string              `yaml:"textfile_directory,omitempty"`
	VMStatFields                     string              `yaml:"vmstat_fields,omitempty"`

	UnmarshalWarnings []string `yaml:"-"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type baseConfig Config
	type config struct {
		baseConfig `yaml:",inline"`

		// Deprecated field names:
		NetdevDeviceWhitelist        string `yaml:"netdev_device_whitelist,omitempty"`
		NetdevDeviceBlacklist        string `yaml:"netdev_device_blacklist,omitempty"`
		SystemdUnitWhitelist         string `yaml:"systemd_unit_whitelist,omitempty"`
		SystemdUnitBlacklist         string `yaml:"systemd_unit_blacklist,omitempty"`
		FilesystemIgnoredMountPoints string `yaml:"filesystem_ignored_mount_points,omitempty"`
		FilesystemIgnoredFSTypes     string `yaml:"filesystem_ignored_fs_types,omitempty"`
		DiskStatsIgnoredDevices      string `yaml:"diskstats_ignored_devices,omitempty"`
	}

	var fc config // our full config (schema + deprecated names)
	fc.baseConfig = baseConfig(*c)

	type migratedField struct {
		OldName, NewName   string
		OldValue, NewValue *string

		defaultValue string
	}
	migratedFields := []*migratedField{
		{
			OldName: "netdev_device_whitelist", NewName: "netdev_device_include",
			OldValue: &fc.NetdevDeviceWhitelist, NewValue: &fc.NetdevDeviceInclude,
		},
		{
			OldName: "netdev_device_blacklist", NewName: "netdev_device_exclude",
			OldValue: &fc.NetdevDeviceBlacklist, NewValue: &fc.NetdevDeviceExclude,
		},
		{
			OldName: "systemd_unit_whitelist", NewName: "systemd_unit_include",
			OldValue: &fc.SystemdUnitWhitelist, NewValue: &fc.SystemdUnitInclude,
		},
		{
			OldName: "systemd_unit_blacklist", NewName: "systemd_unit_exclude",
			OldValue: &fc.SystemdUnitBlacklist, NewValue: &fc.SystemdUnitExclude,
		},
		{
			OldName: "filesystem_ignored_mount_points", NewName: "filesystem_mount_points_exclude",
			OldValue: &fc.FilesystemIgnoredMountPoints, NewValue: &fc.FilesystemMountPointsExclude,
		},
		{
			OldName: "filesystem_ignored_fs_types", NewName: "filesystem_fs_types_exclude",
			OldValue: &fc.FilesystemIgnoredFSTypes, NewValue: &fc.FilesystemFSTypesExclude,
		},
		{
			OldName: "diskstats_ignored_devices", NewName: "diskstats_device_exclude",
			OldValue: &fc.DiskStatsIgnoredDevices, NewValue: &fc.DiskStatsDeviceExclude,
		},
	}

	// We don't know when fields are unmarshaled unless they have non-zero
	// values. Defaults stop us from being able to check, so we'll temporarily
	// cache the default and make sure both the old and new migrated fields are
	// zero.
	for _, mf := range migratedFields {
		mf.defaultValue = *mf.NewValue
		*mf.NewValue = ""
	}

	if err := unmarshal(&fc); err != nil {
		return err
	}

	for _, mf := range migratedFields {
		switch {
		case *mf.OldValue != "" && *mf.NewValue != "": // New set, old set
			return fmt.Errorf("only one of %q and %q may be specified", mf.OldName, mf.NewName)

		case *mf.NewValue == "" && *mf.OldValue != "": // New unset, old set
			*mf.NewValue = *mf.OldValue

			warning := fmt.Sprintf("%q is deprecated by %q and will be removed in a future version", mf.OldName, mf.NewName)
			fc.UnmarshalWarnings = append(fc.UnmarshalWarnings, warning)

		case *mf.NewValue == "" && *mf.OldValue == "": // Neither set.
			// Copy the default back to mf.NewValue.
			*mf.NewValue = mf.defaultValue

		case *mf.NewValue != "" && *mf.OldValue == "": // New set, old unset
			// Nothing to do
		}
	}

	*c = (Config)(fc.baseConfig)
	return nil
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "node_exporter"
}

func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return defaultKey, nil
}

// NewIntegration converts this config into an instance of an integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeSingleton, metricsutils.Shim)
}

func (c *Config) mapConfigToNodeConfig() *collector.NodeCollectorConfig {
	validCollectors := make(map[string]bool)
	collectors := make(map[string]CollectorState, len(Collectors))
	for k, v := range Collectors {
		collectors[k] = v
	}

	// Override the set of defaults with the provided set of collectors if
	// set_collectors has at least one element in it.
	if len(c.SetCollectors) != 0 {
		customDefaults := map[string]struct{}{}
		for _, c := range c.SetCollectors {
			customDefaults[c] = struct{}{}
		}

		for k := range collectors {
			_, shouldEnable := customDefaults[k]
			if shouldEnable {
				collectors[k] = CollectorStateEnabled
			} else {
				collectors[k] = CollectorStateDisabled
			}
		}
	} else {
		// This gets the default enabled passed in via register.
		for k, v := range collector.GetDefaults() {
			collectors[k] = CollectorState(v)
		}
	}
	// Explicitly disable/enable specific collectors
	for _, c := range c.DisableCollectors {
		collectors[c] = CollectorStateDisabled
	}
	for _, c := range c.EnableCollectors {
		collectors[c] = CollectorStateEnabled
	}

	for k, v := range collectors {
		validCollectors[k] = bool(v)
	}

	// This removes any collectors not available on the platform.
	availableCollectors := collector.GetAvailableCollectors()
	for name := range validCollectors {
		var found bool
		for _, availableName := range availableCollectors {
			if name != availableName {
				continue
			}
			found = true
			break
		}
		if !found {
			delete(validCollectors, name)
		}
	}

	// blankString is a hack to emulate the behavior of kingpin, where node_exporter checks for blank string against a pointer
	// without first checking the validity of the pointer.
	// TODO change node_exporter to check for nil first.
	blankString := ""
	blankBool := false
	blankInt := 0

	cfg := &collector.NodeCollectorConfig{}

	// It is safe to set all these configs these since only collectors that are enabled are used.

	cfg.Path = collector.PathConfig{
		ProcPath:     &c.ProcFSPath,
		SysPath:      &c.SysFSPath,
		RootfsPath:   &c.RootFSPath,
		UdevDataPath: &c.UdevDataPath,
	}

	cfg.Bcache = collector.BcacheConfig{
		PriorityStats: &c.BcachePriorityStats,
	}
	cfg.CPU = collector.CPUConfig{
		EnableCPUGuest: &c.CPUEnableCPUGuest,
		EnableCPUInfo:  &c.CPUEnableCPUInfo,
		BugsInclude:    &c.CPUBugsInclude,
		FlagsInclude:   &c.CPUFlagsInclude,
	}
	if c.DiskStatsDeviceInclude != "" {
		cfg.DiskstatsDeviceFilter = collector.DiskstatsDeviceFilterConfig{
			DeviceInclude:    &c.DiskStatsDeviceInclude,
			OldDeviceExclude: &blankString,
			DeviceExclude:    &blankString,
			DeviceExcludeSet: false,
		}
	} else {
		cfg.DiskstatsDeviceFilter = collector.DiskstatsDeviceFilterConfig{
			DeviceExclude:    &c.DiskStatsDeviceExclude,
			DeviceExcludeSet: true,
			OldDeviceExclude: &blankString,
			DeviceInclude:    &blankString,
		}
	}

	cfg.Ethtool = collector.EthtoolConfig{
		DeviceInclude:   &c.EthtoolDeviceInclude,
		DeviceExclude:   &c.EthtoolDeviceExclude,
		IncludedMetrics: &c.EthtoolMetricsInclude,
	}

	cfg.Filesystem = collector.FilesystemConfig{
		MountPointsExclude:     &c.FilesystemMountPointsExclude,
		MountPointsExcludeSet:  true,
		MountTimeout:           &c.FilesystemMountTimeout,
		FSTypesExclude:         &c.FilesystemFSTypesExclude,
		FSTypesExcludeSet:      true,
		OldFSTypesExcluded:     &blankString,
		OldMountPointsExcluded: &blankString,
		StatWorkerCount:        &blankInt,
	}

	cfg.HwMon = collector.HwMonConfig{
		ChipInclude: &c.HwMonChipInclude,
		ChipExclude: &c.HwMonChipExclude,
	}

	var joinedLabels string
	if len(c.IPVSBackendLabels) > 0 {
		joinedLabels = strings.Join(c.IPVSBackendLabels, ",")
		cfg.IPVS = collector.IPVSConfig{
			Labels:    &joinedLabels,
			LabelsSet: true,
		}
	} else {
		cfg.IPVS = collector.IPVSConfig{
			Labels:    &joinedLabels,
			LabelsSet: false,
		}
	}

	cfg.NetClass = collector.NetClassConfig{
		IgnoredDevices: &c.NetclassIgnoredDevices,
		InvalidSpeed:   &c.NetclassIgnoreInvalidSpeedDevice,
		Netlink:        &blankBool,
		RTNLWithStats:  &blankBool,
	}

	cfg.NetDev = collector.NetDevConfig{
		DeviceInclude:    &c.NetdevDeviceInclude,
		DeviceExclude:    &c.NetdevDeviceExclude,
		AddressInfo:      &c.NetdevAddressInfo,
		OldDeviceInclude: &blankString,
		OldDeviceExclude: &blankString,
		Netlink:          &blankBool,
		DetailedMetrics:  &blankBool,
	}

	cfg.NetStat = collector.NetStatConfig{
		Fields: &c.NetstatFields,
	}

	defaultPort := 123
	cfg.NTP = collector.NTPConfig{
		Server:          &c.NTPServer,
		ServerPort:      &defaultPort,
		ProtocolVersion: &c.NTPProtocolVersion,
		IPTTL:           &c.NTPIPTTL,
		MaxDistance:     &c.NTPMaxDistance,
		OffsetTolerance: &c.NTPLocalOffsetTolerance,
		ServerIsLocal:   &c.NTPServerIsLocal,
	}

	cfg.Perf = collector.PerfConfig{
		CPUs:           &c.PerfCPUS,
		Tracepoint:     flagSliceToStringSlice(c.PerfTracepoint),
		NoHwProfiler:   &c.PerfDisableHardwareProfilers,
		HwProfiler:     flagSliceToStringSlice(c.PerfHardwareProfilers),
		NoSwProfiler:   &c.PerfDisableSoftwareProfilers,
		SwProfiler:     flagSliceToStringSlice(c.PerfSoftwareProfilers),
		NoCaProfiler:   &c.PerfDisableCacheProfilers,
		CaProfilerFlag: flagSliceToStringSlice(c.PerfCacheProfilers),
	}

	cfg.PowerSupplyClass = collector.PowerSupplyClassConfig{
		IgnoredPowerSupplies: &c.PowersupplyIgnoredSupplies,
	}

	cfg.Runit = collector.RunitConfig{
		ServiceDir: &c.RunitServiceDir,
	}

	cfg.Supervisord = collector.SupervisordConfig{
		URL: &c.SupervisordURL,
	}

	cfg.Sysctl = collector.SysctlConfig{
		Include:     flagSliceToStringSlice(c.SysctlInclude),
		IncludeInfo: flagSliceToStringSlice(c.SysctlIncludeInfo),
	}

	cfg.Systemd = collector.SystemdConfig{
		UnitInclude:            &c.SystemdUnitInclude,
		UnitIncludeSet:         true,
		UnitExclude:            &c.SystemdUnitExclude,
		UnitExcludeSet:         true,
		EnableTaskMetrics:      &c.SystemdEnableTaskMetrics,
		EnableRestartsMetrics:  &c.SystemdEnableRestartsMetrics,
		EnableStartTimeMetrics: &c.SystemdEnableStartTimeMetrics,
		OldUnitExclude:         &blankString,
		OldUnitInclude:         &blankString,
		Private:                &blankBool,
	}

	cfg.Tapestats = collector.TapestatsConfig{
		IgnoredDevices: &c.TapestatsIgnoredDevices,
	}

	cfg.TextFile = collector.TextFileConfig{
		Directory: &c.TextfileDirectory,
	}

	cfg.VmStat = collector.VmStatConfig{
		Fields: &c.VMStatFields,
	}

	cfg.Arp = collector.ArpConfig{
		DeviceInclude: &blankString,
		DeviceExclude: &blankString,
		Netlink:       &blankBool,
	}

	cfg.Stat = collector.StatConfig{
		Softirq: &blankBool,
	}

	cfg.HwMon = collector.HwMonConfig{
		ChipInclude: &blankString,
		ChipExclude: &blankString,
	}

	cfg.Qdisc = collector.QdiscConfig{
		Fixtures:         &blankString,
		DeviceInclude:    &blankString,
		OldDeviceInclude: &blankString,
		DeviceExclude:    &blankString,
		OldDeviceExclude: &blankString,
	}

	cfg.Rapl = collector.RaplConfig{
		ZoneLabel: &blankBool,
	}

	cfg.Systemd = collector.SystemdConfig{
		UnitInclude:            &c.SystemdUnitInclude,
		UnitIncludeSet:         true,
		UnitExclude:            &c.SystemdUnitExclude,
		UnitExcludeSet:         true,
		OldUnitInclude:         &blankString,
		OldUnitExclude:         &blankString,
		Private:                &blankBool,
		EnableTaskMetrics:      &c.SystemdEnableTaskMetrics,
		EnableRestartsMetrics:  &c.SystemdEnableRestartsMetrics,
		EnableStartTimeMetrics: &c.SystemdEnableStartTimeMetrics,
	}

	cfg.Wifi = collector.WifiConfig{
		Fixtures: &blankString,
	}

	cfg.Collectors = validCollectors

	return cfg
}

func flagSliceToStringSlice(fl flagext.StringSlice) *[]string {
	sl := make([]string, len(fl))
	copy(sl, fl)
	return &sl
}
