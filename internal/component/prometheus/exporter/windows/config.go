package windows

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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
	Filetime           FiletimeConfig           `alloy:"filetime,block,optional"`
	PerformanceCounter PerformanceCounterConfig `alloy:"performancecounter,block,optional"`
	MSCluster          MSClusterConfig          `alloy:"mscluster,block,optional"`
	NetFramework       NetFrameworkConfig       `alloy:"netframework,block,optional"`
	DNS                DNSConfig                `alloy:"dns,block,optional"`
	Net                NetConfig                `alloy:"net,block,optional"`
}

// Convert converts the component's Arguments to the integration's Config.
func (a *Arguments) Convert(logger log.Logger) *windows_integration.Config {
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
		Filetime:           a.Filetime.Convert(),
		PerformanceCounter: a.PerformanceCounter.Convert(),
		MSCluster:          a.MSCluster.Convert(),
		NetFramework:       a.NetFramework.Convert(),
		Update:             a.Update.Convert(),
		DNS:                a.DNS.Convert(),
		Net:                a.Net.Convert(),
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

func (a *Arguments) logDeprecatedFields(logger log.Logger) {
	if slices.Contains(a.EnabledCollectors, "cs") {
		level.Warn(logger).Log("msg", "the `cs` collector is removed - its metrics are in the `os`, `memory`, and `cpu` collectors")
	}
	if slices.Contains(a.EnabledCollectors, "logon") {
		level.Warn(logger).Log("msg", "the `logon` collector is removed - its metrics are in the `terminal_services` collector")
	}
	if a.MSMQ != nil {
		level.Warn(logger).Log("msg", "the `msmq` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.Service.UseApi != nil {
		level.Warn(logger).Log("msg", "the `use_api` attribute inside the `service` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.Service.Where != nil {
		level.Warn(logger).Log("msg", "the `where_clause` attribute inside the `service` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.Service.V2 != nil {
		level.Warn(logger).Log("msg", "the `enable_v2_collector` attribute inside the `service` block is deprecated - its usage is a no-op and it will be removed in the future")
	}
	if a.TextFileDeprecated != nil {
		level.Warn(logger).Log("msg", "the `text_file` block is deprecated and will be removed in the future - use `textfile` instead")
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
	CounterVersion         uint8  `alloy:"counter_version,attr,optional"`
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
	Exclude string `alloy:"exclude,attr,optional"`
	Include string `alloy:"include,attr,optional"`
}

// Convert converts the component's ScheduledTaskConfig to the integration's ScheduledTaskConfig.
func (t ScheduledTaskConfig) Convert() windows_integration.ScheduledTaskConfig {
	return windows_integration.ScheduledTaskConfig{
		Exclude: wrapRegex(t.Exclude),
		Include: wrapRegex(t.Include),
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

// FiletimeConfig handles settings for the windows_exporter filetime collector
type FiletimeConfig struct {
	FilePatterns []string `alloy:"file_patterns,attr,optional"`
}

// Convert converts the component's FiletimeConfig to the integration's FiletimeConfig.
func (t FiletimeConfig) Convert() windows_integration.FiletimeConfig {
	return windows_integration.FiletimeConfig{
		FilePatterns: t.FilePatterns,
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
