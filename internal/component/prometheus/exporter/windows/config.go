package windows

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	windows_integration "github.com/grafana/alloy/internal/static/integrations/windows_exporter"
)

// Wrap some regex strings to prevent issues with user-supplied empty strings.
// Prior to v0.27, the upstream exporter used to wrap regexes like this.
// Alloy is now doing this instead, to maintain backwards compatibility.
//
// This is mostly to prevent issues with `exclude` arguments.
// If `exclude` is set to `""` and there is no wrapping, the regex will match everything.
// Therefore, all collectors will be excluded.
//
// See https://github.com/grafana/alloy/issues/1845
// TODO: Remove this in Alloy v2.
func wrapRegex(regex string) string {
	return fmt.Sprintf("^(?:%s)$", regex)
}

// Arguments is used for controlling for this exporter.
type Arguments struct {
	// Collectors to mark as enabled
	EnabledCollectors []string `alloy:"enabled_collectors,attr,optional"`

	// Collector-specific config options
	Dfsr               DfsrConfig               `alloy:"dfsr,block,optional"`
	Exchange           ExchangeConfig           `alloy:"exchange,block,optional"`
	IIS                IISConfig                `alloy:"iis,block,optional"`
	LogicalDisk        LogicalDiskConfig        `alloy:"logical_disk,block,optional"`
	MSMQ               *MSMQConfig              `alloy:"msmq,block,optional"`
	MSSQL              MSSQLConfig              `alloy:"mssql,block,optional"`
	Network            NetworkConfig            `alloy:"network,block,optional"`
	PhysicalDisk       PhysicalDiskConfig       `alloy:"physical_disk,block,optional"`
	Printer            PrinterConfig            `alloy:"printer,block,optional"`
	Process            ProcessConfig            `alloy:"process,block,optional"`
	ScheduledTask      ScheduledTaskConfig      `alloy:"scheduled_task,block,optional"`
	Service            ServiceConfig            `alloy:"service,block,optional"`
	SMB                SMBConfig                `alloy:"smb,block,optional"`
	SMBClient          SMBClientConfig          `alloy:"smb_client,block,optional"`
	SMTP               SMTPConfig               `alloy:"smtp,block,optional"`
	TextFileDeprecated *TextFileConfig          `alloy:"text_file,block,optional"`
	TextFile           *TextFileConfig          `alloy:"textfile,block,optional"`
	TCP                TCPConfig                `alloy:"tcp,block,optional"`
	Update             UpdateConfig             `alloy:"update,block,optional"`
	File               FileConfig               `alloy:"file,block,optional"`
	Filetime           FiletimeConfig           `alloy:"filetime,block,optional"`
	PerformanceCounter PerformanceCounterConfig `alloy:"performancecounter,block,optional"`
	Container          ContainerConfig          `alloy:"container,block,optional"`
	Registry           RegistryConfig           `alloy:"registry,block,optional"`
	WMI                WMIConfig                `alloy:"wmi,block,optional"`
	MSCluster          MSClusterConfig          `alloy:"mscluster,block,optional"`
	NetFramework       NetFrameworkConfig       `alloy:"netframework,block,optional"`
	DNS                DNSConfig                `alloy:"dns,block,optional"`
	Net                NetConfig                `alloy:"net,block,optional"`
	Time               TimeConfig               `alloy:"time,block,optional"`
	HyperV             HyperVConfig             `alloy:"hyperv,block,optional"`
	DHCP               DHCPConfig               `alloy:"dhcp,block,optional"`
	StorageSpaces      StorageSpacesConfig      `alloy:"storage_spaces,block,optional"`
}

// Convert converts the component's Arguments to the integration's Config.
func (a *Arguments) Convert(logger *slog.Logger) *windows_integration.Config {
	a.logDeprecatedFields(logger)

	filteredCollectors := slices.DeleteFunc(a.EnabledCollectors, func(collector string) bool {
		return collector == "cs" || collector == "logon"
	})

	return &windows_integration.Config{
		EnabledCollectors:  strings.Join(filteredCollectors, ","),
		Dfsr:               a.Dfsr.Convert(),
		Exchange:           a.Exchange.Convert(),
		IIS:                a.IIS.Convert(),
		LogicalDisk:        a.LogicalDisk.Convert(),
		MSSQL:              a.MSSQL.Convert(),
		Network:            a.Network.Convert(),
		Process:            a.Process.Convert(),
		PhysicalDisk:       a.PhysicalDisk.Convert(),
		Printer:            a.Printer.Convert(),
		ScheduledTask:      a.ScheduledTask.Convert(),
		Service:            a.Service.Convert(),
		SMB:                a.SMB.Convert(),
		SMBClient:          a.SMBClient.Convert(),
		SMTP:               a.SMTP.Convert(),
		TextFile:           CombineAndConvert(a.TextFile, a.TextFileDeprecated),
		TCP:                a.TCP.Convert(),
		File:               a.File.Convert(),
		Filetime:           a.Filetime.Convert(),
		PerformanceCounter: a.PerformanceCounter.Convert(),
		Container:          a.Container.Convert(),
		Registry:           a.Registry.Convert(),
		WMI:                a.WMI.Convert(),
		MSCluster:          a.MSCluster.Convert(),
		NetFramework:       a.NetFramework.Convert(),
		Update:             a.Update.Convert(),
		DNS:                a.DNS.Convert(),
		Net:                a.Net.Convert(),
		Time:               a.Time.Convert(),
		HyperV:             a.HyperV.Convert(),
		DHCP:               a.DHCP.Convert(),
		StorageSpaces:      a.StorageSpaces.Convert(),
	}
}

// Combine combines two TextFileConfig instances to ensure support for the deprecated version
func CombineAndConvert(t, deprecated *TextFileConfig) windows_integration.TextFileConfig {
	if t == nil && deprecated == nil {
		return windows_integration.TextFileConfig{}
	}

	if t == nil {
		return deprecated.Convert()
	}

	if deprecated == nil {
		return t.Convert()
	}

	directories := append(t.Directories, deprecated.Directories...)
	// Support the deprecated `text_file_directory` attribute.
	if len(t.TextFileDirectory) > 0 {
		directories = append(directories, strings.Split(t.TextFileDirectory, ",")...)
	}
	if len(deprecated.TextFileDirectory) > 0 {
		directories = append(directories, strings.Split(deprecated.TextFileDirectory, ",")...)
	}
	// Sort so we can use slices.Compact to remove duplicates.
	slices.Sort(directories)
	return windows_integration.TextFileConfig{
		TextFileDirectory: strings.Join(slices.Compact(directories), ","),
	}
}

func (a *Arguments) logDeprecatedFields(logger *slog.Logger) {
	if slices.Contains(a.EnabledCollectors, "cs") {
		logger.Warn("the `cs` collector is removed - its metrics are in the `os`, `memory`, and `cpu` collectors")
	}
	if slices.Contains(a.EnabledCollectors, "logon") {
		logger.Warn("the `logon` collector is removed - its metrics are in the `terminal_services` collector")
	}
	if slices.Contains(a.EnabledCollectors, "filetime") || len(a.Filetime.FilePatterns) > 0 {
		logger.Warn("the `filetime` collector and block are deprecated - Alloy runs the `file` collector instead, which also exports file size. Use the `file` collector and block")
	}
	if slices.Contains(a.EnabledCollectors, "thermalzone") {
		logger.Warn("the `thermalzone` collector is removed in windows_exporter v0.32 - Alloy ignores it. Use the `performancecounter` block to read the `Thermal Zone Information` counters")
	}
	if a.Process.CounterVersion != 0 {
		logger.Warn("the `counter_version` attribute inside the `process` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.MSMQ != nil {
		logger.Warn("the `msmq` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.Service.UseApi != nil {
		logger.Warn("the `use_api` attribute inside the `service` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.Service.Where != nil {
		logger.Warn("the `where_clause` attribute inside the `service` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.Service.V2 != nil {
		logger.Warn("the `enable_v2_collector` attribute inside the `service` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.TextFileDeprecated != nil {
		logger.Warn("the `text_file` block is deprecated and will be removed in the future - use `textfile` instead")
	}
}

// DfsrConfig handles settings for the windows_exporter Exchange collector
type DfsrConfig struct {
	SourcesEnabled []string `alloy:"sources_enabled,attr,optional"`
}

// Convert converts the component's DfsrConfig to the integration's ExchangeConfig.
func (t DfsrConfig) Convert() windows_integration.DfsrConfig {
	return windows_integration.DfsrConfig{
		SourcesEnabled: strings.Join(t.SourcesEnabled, ","),
	}
}

// ExchangeConfig handles settings for the windows_exporter Exchange collector
type ExchangeConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's ExchangeConfig to the integration's ExchangeConfig.
func (t ExchangeConfig) Convert() windows_integration.ExchangeConfig {
	return windows_integration.ExchangeConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// IISConfig handles settings for the windows_exporter IIS collector
type IISConfig struct {
	AppBlackList  string `alloy:"app_blacklist,attr,optional"`
	AppWhiteList  string `alloy:"app_whitelist,attr,optional"`
	SiteBlackList string `alloy:"site_blacklist,attr,optional"`
	SiteWhiteList string `alloy:"site_whitelist,attr,optional"`
	AppExclude    string `alloy:"app_exclude,attr,optional"`
	AppInclude    string `alloy:"app_include,attr,optional"`
	SiteExclude   string `alloy:"site_exclude,attr,optional"`
	SiteInclude   string `alloy:"site_include,attr,optional"`
}

// Convert converts the component's IISConfig to the integration's IISConfig.
func (t IISConfig) Convert() windows_integration.IISConfig {
	return windows_integration.IISConfig{
		AppBlackList:  wrapRegex(t.AppBlackList),
		AppWhiteList:  wrapRegex(t.AppWhiteList),
		SiteBlackList: wrapRegex(t.SiteBlackList),
		SiteWhiteList: wrapRegex(t.SiteWhiteList),
		AppExclude:    wrapRegex(t.AppExclude),
		AppInclude:    wrapRegex(t.AppInclude),
		SiteExclude:   wrapRegex(t.SiteExclude),
		SiteInclude:   wrapRegex(t.SiteInclude),
	}
}

// TextFileConfig handles settings for the windows_exporter Text File collector
type TextFileConfig struct {
	TextFileDirectory string   `alloy:"text_file_directory,attr,optional"`
	Directories       []string `alloy:"directories,attr,optional"`
}

// Convert converts the component's TextFileConfig to the integration's TextFileConfig.
func (t TextFileConfig) Convert() windows_integration.TextFileConfig {
	directories := t.Directories
	// Support the deprecated `text_file_directory` attribute.
	if len(t.TextFileDirectory) > 0 {
		directories = append(directories, strings.Split(t.TextFileDirectory, ",")...)
		slices.Sort(directories)
		directories = slices.Compact(directories)
	}
	return windows_integration.TextFileConfig{
		TextFileDirectory: strings.Join(directories, ","),
	}
}

// SMTPConfig handles settings for the windows_exporter SMTP collector
type SMTPConfig struct {
	BlackList string `alloy:"blacklist,attr,optional"`
	WhiteList string `alloy:"whitelist,attr,optional"`
	Exclude   string `alloy:"exclude,attr,optional"`
	Include   string `alloy:"include,attr,optional"`
}

// Convert converts the component's SMTPConfig to the integration's SMTPConfig.
func (t SMTPConfig) Convert() windows_integration.SMTPConfig {
	return windows_integration.SMTPConfig{
		BlackList: wrapRegex(t.BlackList),
		WhiteList: wrapRegex(t.WhiteList),
		Exclude:   wrapRegex(t.Exclude),
		Include:   wrapRegex(t.Include),
	}
}

// ServiceConfig handles settings for the windows_exporter service collector
type ServiceConfig struct {
	Include string  `alloy:"include,attr,optional"`
	Exclude string  `alloy:"exclude,attr,optional"`
	UseApi  *string `alloy:"use_api,attr,optional"`
	Where   *string `alloy:"where_clause,attr,optional"`
	V2      *string `alloy:"enable_v2_collector,attr,optional"`
}

// Convert converts the component's ServiceConfig to the integration's ServiceConfig.
func (t ServiceConfig) Convert() windows_integration.ServiceConfig {
	return windows_integration.ServiceConfig{
		Include: wrapRegex(t.Include),
		Exclude: wrapRegex(t.Exclude),
	}
}

// ProcessConfig handles settings for the windows_exporter process collector
type ProcessConfig struct {
	BlackList              string `alloy:"blacklist,attr,optional"`
	WhiteList              string `alloy:"whitelist,attr,optional"`
	Exclude                string `alloy:"exclude,attr,optional"`
	Include                string `alloy:"include,attr,optional"`
	EnableIISWorkerProcess bool   `alloy:"enable_iis_worker_process,attr,optional"`
	// CounterVersion is deprecated. It has no effect.
	CounterVersion uint8 `alloy:"counter_version,attr,optional"`
}

// Convert converts the component's ProcessConfig to the integration's ProcessConfig.
func (t ProcessConfig) Convert() windows_integration.ProcessConfig {
	return windows_integration.ProcessConfig{
		BlackList:              wrapRegex(t.BlackList),
		WhiteList:              wrapRegex(t.WhiteList),
		Exclude:                wrapRegex(t.Exclude),
		Include:                wrapRegex(t.Include),
		EnableIISWorkerProcess: t.EnableIISWorkerProcess,
		CounterVersion:         t.CounterVersion,
	}
}

// ScheduledTaskConfig handles settings for the windows_exporter process collector
type ScheduledTaskConfig struct {
	Exclude    string `alloy:"exclude,attr,optional"`
	Include    string `alloy:"include,attr,optional"`
	MissedRuns bool   `alloy:"missed_runs,attr,optional"`
}

// Convert converts the component's ScheduledTaskConfig to the integration's ScheduledTaskConfig.
func (t ScheduledTaskConfig) Convert() windows_integration.ScheduledTaskConfig {
	return windows_integration.ScheduledTaskConfig{
		Exclude:    wrapRegex(t.Exclude),
		Include:    wrapRegex(t.Include),
		MissedRuns: t.MissedRuns,
	}
}

// NetworkConfig handles settings for the windows_exporter network collector
type NetworkConfig struct {
	BlackList string `alloy:"blacklist,attr,optional"`
	WhiteList string `alloy:"whitelist,attr,optional"`
	Exclude   string `alloy:"exclude,attr,optional"`
	Include   string `alloy:"include,attr,optional"`
}

// Convert converts the component's NetworkConfig to the integration's NetworkConfig.
func (t NetworkConfig) Convert() windows_integration.NetworkConfig {
	return windows_integration.NetworkConfig{
		BlackList: wrapRegex(t.BlackList),
		WhiteList: wrapRegex(t.WhiteList),
		Exclude:   wrapRegex(t.Exclude),
		Include:   wrapRegex(t.Include),
	}
}

// NetConfig handles settings for the windows_exporter net collector
type NetConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
	Exclude     string   `alloy:"exclude,attr,optional"`
	Include     string   `alloy:"include,attr,optional"`
}

// Convert converts the component's NetConfig to the integration's NetConfig.
func (t NetConfig) Convert() windows_integration.NetConfig {
	return windows_integration.NetConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
		Exclude:     wrapRegex(t.Exclude),
		Include:     wrapRegex(t.Include),
	}
}

// MSSQLConfig handles settings for the windows_exporter SQL server collector
type MSSQLConfig struct {
	EnabledClasses []string `alloy:"enabled_classes,attr,optional"`
}

// Convert converts the component's MSSQLConfig to the integration's MSSQLConfig.
func (t MSSQLConfig) Convert() windows_integration.MSSQLConfig {
	return windows_integration.MSSQLConfig{
		EnabledClasses: strings.Join(t.EnabledClasses, ","),
	}
}

type MSMQConfig struct {
	Where string `alloy:"where_clause,attr,optional"`
}

// LogicalDiskConfig handles settings for the windows_exporter logical disk collector
type LogicalDiskConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
	BlackList   string   `alloy:"blacklist,attr,optional"`
	WhiteList   string   `alloy:"whitelist,attr,optional"`
	Include     string   `alloy:"include,attr,optional"`
	Exclude     string   `alloy:"exclude,attr,optional"`
}

// Convert converts the component's LogicalDiskConfig to the integration's LogicalDiskConfig.
func (t LogicalDiskConfig) Convert() windows_integration.LogicalDiskConfig {
	return windows_integration.LogicalDiskConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
		BlackList:   wrapRegex(t.BlackList),
		WhiteList:   wrapRegex(t.WhiteList),
		Include:     wrapRegex(t.Include),
		Exclude:     wrapRegex(t.Exclude),
	}
}

// PhysicalDiskConfig handles settings for the windows_exporter physical disk collector
type PhysicalDiskConfig struct {
	Include string `alloy:"include,attr,optional"`
	Exclude string `alloy:"exclude,attr,optional"`
}

// Convert converts the component's PhysicalDiskConfig to the integration's PhysicalDiskConfig.
func (t PhysicalDiskConfig) Convert() windows_integration.PhysicalDiskConfig {
	return windows_integration.PhysicalDiskConfig{
		Include: wrapRegex(t.Include),
		Exclude: wrapRegex(t.Exclude),
	}
}

// PrinterConfig handles settings for the windows_exporter printer collector
type PrinterConfig struct {
	Exclude string `alloy:"exclude,attr,optional"`
	Include string `alloy:"include,attr,optional"`
}

// Convert converts the component's PrinterConfig to the integration's PrinterConfig.
func (t PrinterConfig) Convert() windows_integration.PrinterConfig {
	return windows_integration.PrinterConfig{
		Exclude: wrapRegex(t.Exclude),
		Include: wrapRegex(t.Include),
	}
}

// SMBConfig handles settings for the windows_exporter smb collector
type SMBConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's ExchangeConfig to the integration's ExchangeConfig.
func (t SMBConfig) Convert() windows_integration.SMBConfig {
	return windows_integration.SMBConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// SMBClientConfig handles settings for the windows_exporter smb client collector
type SMBClientConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's ExchangeConfig to the integration's ExchangeConfig.
func (t SMBClientConfig) Convert() windows_integration.SMBClientConfig {
	return windows_integration.SMBClientConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// TCPConfig handles settings for the windows_exporter TCP collector
type TCPConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's TCPConfig to the integration's TCPConfig.
func (t TCPConfig) Convert() windows_integration.TCPConfig {
	return windows_integration.TCPConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// UpdateConfig handles settings for the windows_exporter Update collector
type UpdateConfig struct {
	Online         bool          `alloy:"online,attr,optional"`
	ScrapeInterval time.Duration `alloy:"scrape_interval,attr,optional"`
}

// Convert converts the component's UpdateConfig to the integration's UpdateConfig.
func (t UpdateConfig) Convert() windows_integration.UpdateConfig {
	return windows_integration.UpdateConfig{
		Online:         t.Online,
		ScrapeInterval: t.ScrapeInterval,
	}
}

// FileConfig handles settings for the windows_exporter file collector
type FileConfig struct {
	FilePatterns []string `alloy:"file_patterns,attr,optional"`
}

// Convert converts the component's FileConfig to the integration's FileConfig.
func (t FileConfig) Convert() windows_integration.FileConfig {
	return windows_integration.FileConfig{
		FilePatterns: t.FilePatterns,
	}
}

// FiletimeConfig handles settings for the filetime collector.
// Deprecated: windows_exporter v0.32 replaced the filetime collector with the file collector.
// Use FileConfig instead.
type FiletimeConfig struct {
	FilePatterns []string `alloy:"file_patterns,attr,optional"`
}

// Convert converts the component's FiletimeConfig to the integration's FiletimeConfig.
func (t FiletimeConfig) Convert() windows_integration.FiletimeConfig {
	return windows_integration.FiletimeConfig{
		FilePatterns: t.FilePatterns,
	}
}

// ContainerConfig handles settings for the windows_exporter container collector
type ContainerConfig struct {
	CRIEndpoint string `alloy:"cri_endpoint,attr,optional"`
}

// Convert converts the component's ContainerConfig to the integration's ContainerConfig.
func (t ContainerConfig) Convert() windows_integration.ContainerConfig {
	return windows_integration.ContainerConfig{
		CRIEndpoint: t.CRIEndpoint,
	}
}

// RegistryConfig handles settings for the windows_exporter registry collector
type RegistryConfig struct {
	Keys string `alloy:"keys,attr,optional"`
}

// Convert converts the component's RegistryConfig to the integration's RegistryConfig.
func (t RegistryConfig) Convert() windows_integration.RegistryConfig {
	return windows_integration.RegistryConfig{
		Keys: t.Keys,
	}
}

// WMIConfig handles settings for the windows_exporter wmi collector
type WMIConfig struct {
	Queries string `alloy:"queries,attr,optional"`
}

// Convert converts the component's WMIConfig to the integration's WMIConfig.
func (t WMIConfig) Convert() windows_integration.WMIConfig {
	return windows_integration.WMIConfig{
		Queries: t.Queries,
	}
}

// PerformanceCounterConfig handles settings for the windows_exporter performance counter collector
type PerformanceCounterConfig struct {
	Objects string `alloy:"objects,attr,optional"`
}

// Convert converts the component's PerformanceCounterConfig to the integration's PerformanceCounterConfig.
func (t PerformanceCounterConfig) Convert() windows_integration.PerformanceCounterConfig {
	return windows_integration.PerformanceCounterConfig{
		Objects: t.Objects,
	}
}

// MSClusterConfig handles settings for the windows_exporter MSCluster collector
type MSClusterConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's MSClusterConfig to the integration's MSClusterConfig.
func (t MSClusterConfig) Convert() windows_integration.MSClusterConfig {
	return windows_integration.MSClusterConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// NetFrameworkConfig handles settings for the windows_exporter NetFramework collector
type NetFrameworkConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's NetFrameworkConfig to the integration's NetFrameworkConfig.
func (t NetFrameworkConfig) Convert() windows_integration.NetFrameworkConfig {
	return windows_integration.NetFrameworkConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// DNSConfig handles settings for the windows_exporter DNS collector
type DNSConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's DNSConfig to the integration's DNSConfig.
func (t DNSConfig) Convert() windows_integration.DNSConfig {
	return windows_integration.DNSConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// TimeConfig handles settings for the windows_exporter time collector
type TimeConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's TimeConfig to the integration's TimeConfig.
func (t TimeConfig) Convert() windows_integration.TimeConfig {
	return windows_integration.TimeConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// HyperVConfig handles settings for the windows_exporter hyperv collector
type HyperVConfig struct {
	EnabledList                    []string      `alloy:"enabled_list,attr,optional"`
	VirtualHardDiskRefreshInterval time.Duration `alloy:"virtual_hard_disk_refresh_interval,attr,optional"`
}

// Convert converts the component's HyperVConfig to the integration's HyperVConfig.
func (t HyperVConfig) Convert() windows_integration.HyperVConfig {
	return windows_integration.HyperVConfig{
		EnabledList:                    strings.Join(t.EnabledList, ","),
		VirtualHardDiskRefreshInterval: t.VirtualHardDiskRefreshInterval,
	}
}

// DHCPConfig handles settings for the windows_exporter dhcp collector
type DHCPConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's DHCPConfig to the integration's DHCPConfig.
func (t DHCPConfig) Convert() windows_integration.DHCPConfig {
	return windows_integration.DHCPConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}

// StorageSpacesConfig handles settings for the windows_exporter storage_spaces collector
type StorageSpacesConfig struct {
	EnabledList []string `alloy:"enabled_list,attr,optional"`
}

// Convert converts the component's StorageSpacesConfig to the integration's StorageSpacesConfig.
func (t StorageSpacesConfig) Convert() windows_integration.StorageSpacesConfig {
	return windows_integration.StorageSpacesConfig{
		EnabledList: strings.Join(t.EnabledList, ","),
	}
}
