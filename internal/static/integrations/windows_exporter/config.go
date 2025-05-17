package windows_exporter

import (
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

	Dfsr          DfsrConfig          `yaml:"dfsr,omitempty"`
	Exchange      ExchangeConfig      `yaml:"exchange,omitempty"`
	IIS           IISConfig           `yaml:"iis,omitempty"`
	TextFile      TextFileConfig      `yaml:"text_file,omitempty"`
	SMTP          SMTPConfig          `yaml:"smtp,omitempty"`
	Service       ServiceConfig       `yaml:"service,omitempty"`
	PhysicalDisk  PhysicalDiskConfig  `yaml:"physical_disk,omitempty"`
	Process       ProcessConfig       `yaml:"process,omitempty"`
	Network       NetworkConfig       `yaml:"network,omitempty"`
	MSSQL         MSSQLConfig         `yaml:"mssql,omitempty"`
	MSMQ          MSMQConfig          `yaml:"msmq,omitempty"`
	LogicalDisk   LogicalDiskConfig   `yaml:"logical_disk,omitempty"`
	ScheduledTask ScheduledTaskConfig `yaml:"scheduled_task,omitempty"`
	Printer       PrinterConfig       `yaml:"printer,omitempty"`
	SMB           SMBConfig           `yaml:"smb,omitempty"`
	SMBClient     SMBClientConfig     `yaml:"smb_client,omitempty"`
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
	UseApi string `yaml:"use_api,omitempty"`
	Where  string `yaml:"where_clause,omitempty"`
	V2     string `yaml:"enable_v2_collector,omitempty"`
}

// ProcessConfig handles settings for the windows_exporter process collector
type ProcessConfig struct {
	BlackList string `yaml:"blacklist,omitempty"`
	WhiteList string `yaml:"whitelist,omitempty"`
	Include   string `yaml:"include,omitempty"`
	Exclude   string `yaml:"exclude,omitempty"`
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

// MSMQConfig handles settings for the windows_exporter MSMQ collector
type MSMQConfig struct {
	Where string `yaml:"where_clause,omitempty"`
}

// LogicalDiskConfig handles settings for the windows_exporter logical disk collector
type LogicalDiskConfig struct {
	BlackList string `yaml:"blacklist,omitempty"`
	WhiteList string `yaml:"whitelist,omitempty"`
	Include   string `yaml:"include,omitempty"`
	Exclude   string `yaml:"exclude,omitempty"`
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
