package windows_exporter

import (
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeSingleton, metricsutils.NewNamedShim("windows"))
}

// Config controls the windows_exporter integration.
// All of these and their child fields are pointers, so we can determine if the value was set or not.
type Config struct {
	EnabledCollectors string `yaml:"enabled_collectors"`

	Dfsr               DfsrConfig               `yaml:"dfsr,omitempty"`
	Exchange           ExchangeConfig           `yaml:"exchange,omitempty"`
	MSCluster          MSClusterConfig          `yaml:"mscluster,omitempty"`
	NetFramework       NetFrameworkConfig       `yaml:"netframework,omitempty"`
	IIS                IISConfig                `yaml:"iis,omitempty"`
	TextFile           TextFileConfig           `yaml:"text_file,omitempty"`
	SMTP               SMTPConfig               `yaml:"smtp,omitempty"`
	Service            ServiceConfig            `yaml:"service,omitempty"`
	PhysicalDisk       PhysicalDiskConfig       `yaml:"physical_disk,omitempty"`
	Process            ProcessConfig            `yaml:"process,omitempty"`
	Network            NetworkConfig            `yaml:"network,omitempty"`
	MSSQL              MSSQLConfig              `yaml:"mssql,omitempty"`
	LogicalDisk        LogicalDiskConfig        `yaml:"logical_disk,omitempty"`
	ScheduledTask      ScheduledTaskConfig      `yaml:"scheduled_task,omitempty"`
	Printer            PrinterConfig            `yaml:"printer,omitempty"`
	SMB                SMBConfig                `yaml:"smb,omitempty"`
	SMBClient          SMBClientConfig          `yaml:"smb_client,omitempty"`
	TCP                TCPConfig                `yaml:"tcp,omitempty"`
	Update             UpdateConfig             `yaml:"update,omitempty"`
	Filetime           FiletimeConfig           `yaml:"filetime,omitempty"`
	PerformanceCounter PerformanceCounterConfig `yaml:"performancecounter,omitempty"`
	DNS                DNSConfig                `yaml:"dns,omitempty"`
	Net                NetConfig                `yaml:"net,omitempty"`
}

// Name returns the name used, "windows_explorer"
func (c *Config) Name() string {
	return "windows_exporter"
}

func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return defaultKey, nil
}

// NewIntegration creates an integration based on the given configuration
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

// DfsrConfig handles settings for the windows_exporter dfsr collector
type DfsrConfig struct {
	SourcesEnabled string `yaml:"sources_enabled,omitempty"`
}

// ExchangeConfig handles settings for the windows_exporter Exchange collector
type ExchangeConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
}

// MSClusterConfig handles settings for the windows_exporter MSCluster collector
type MSClusterConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
}

// NetFrameworkConfig handles settings for the windows_exporter NetFramework collector
type NetFrameworkConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
}

// DNSConfig handles settings for the windows_exporter DNS collector
type DNSConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
}

// TCPConfig handles settings for the windows_exporter TCP collector
type TCPConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
}

// UpdateConfig handles settings for the windows_exporter Update collector
type UpdateConfig struct {
	Online         bool          `yaml:"online,omitempty"`
	ScrapeInterval time.Duration `yaml:"scrape_interval,omitempty"`
}

// FiletimeConfig handles settings for the windows_exporter filetime collector
type FiletimeConfig struct {
	FilePatterns []string `yaml:"file_patterns,omitempty"`
}

// PerformanceCounterConfig handles settings for the windows_exporter performance counter collector
type PerformanceCounterConfig struct {
	Objects string `yaml:"objects,omitempty"`
}

// IISConfig handles settings for the windows_exporter IIS collector
type IISConfig struct {
	SiteWhiteList string `yaml:"site_whitelist,omitempty"`
	SiteBlackList string `yaml:"site_blacklist,omitempty"`
	AppWhiteList  string `yaml:"app_whitelist,omitempty"`
	AppBlackList  string `yaml:"app_blacklist,omitempty"`
	SiteInclude   string `yaml:"site_include,omitempty"`
	SiteExclude   string `yaml:"site_exclude,omitempty"`
	AppInclude    string `yaml:"app_include,omitempty"`
	AppExclude    string `yaml:"app_exclude,omitempty"`
}

// TextFileConfig handles settings for the windows_exporter Text File collector
type TextFileConfig struct {
	TextFileDirectory string `yaml:"text_file_directory,omitempty"`
}

// SMTPConfig handles settings for the windows_exporter SMTP collector
type SMTPConfig struct {
	BlackList string `yaml:"blacklist,omitempty"`
	WhiteList string `yaml:"whitelist,omitempty"`
	Include   string `yaml:"include,omitempty"`
	Exclude   string `yaml:"exclude,omitempty"`
}

// ServiceConfig handles settings for the windows_exporter service collector
type ServiceConfig struct {
	Include string `yaml:"include,omitempty"`
	Exclude string `yaml:"exclude,omitempty"`
}

// ProcessConfig handles settings for the windows_exporter process collector
type ProcessConfig struct {
	BlackList              string `yaml:"blacklist,omitempty"`
	WhiteList              string `yaml:"whitelist,omitempty"`
	Include                string `yaml:"include,omitempty"`
	Exclude                string `yaml:"exclude,omitempty"`
	EnableIISWorkerProcess bool   `yaml:"enable_iis_worker_process,omitempty"`
	CounterVersion         uint8  `yaml:"counter_version,omitempty"` // 0 for autoselect, 1 for v1, 2 for v2
}

// NetworkConfig handles settings for the windows_exporter network collector
type NetworkConfig struct {
	BlackList string `yaml:"blacklist,omitempty"`
	WhiteList string `yaml:"whitelist,omitempty"`
	Include   string `yaml:"include,omitempty"`
	Exclude   string `yaml:"exclude,omitempty"`
}

// MSSQLConfig handles settings for the windows_exporter SQL server collector
type MSSQLConfig struct {
	EnabledClasses string `yaml:"enabled_classes,omitempty"`
}

// LogicalDiskConfig handles settings for the windows_exporter logical disk collector
type LogicalDiskConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
	BlackList   string `yaml:"blacklist,omitempty"`
	WhiteList   string `yaml:"whitelist,omitempty"`
	Include     string `yaml:"include,omitempty"`
	Exclude     string `yaml:"exclude,omitempty"`
}

// ScheduledTaskConfig handles settings for the windows_exporter scheduled_task collector
type ScheduledTaskConfig struct {
	Include string `yaml:"include,omitempty"`
	Exclude string `yaml:"exclude,omitempty"`
}

// PhysicalDiskConfig handles settings for the windows_exporter physical disk collector
type PhysicalDiskConfig struct {
	Include string `yaml:"include,omitempty"`
	Exclude string `yaml:"exclude,omitempty"`
}

// PrinterConfig handles settings for the windows_exporter printer collector
type PrinterConfig struct {
	Include string `yaml:"include,omitempty"`
	Exclude string `yaml:"exclude,omitempty"`
}

// SMBConfig handles settings for the windows_exporter smb collector
// Deprecated: This is not used by the windows_exporter
type SMBConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
}

// SMBClientConfig handles settings for the windows_exporter smb client collector
// Deprecated: This is not used by the windows_exporter
type SMBClientConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
}

// NetConfig handles settings for the windows_exporter net collector
type NetConfig struct {
	EnabledList string `yaml:"enabled_list,omitempty"`
	Exclude     string `yaml:"exclude,omitempty"`
	Include     string `yaml:"include,omitempty"`
}
