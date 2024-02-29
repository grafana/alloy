package windows_exporter

import (
	"github.com/prometheus-community/windows_exporter/pkg/collector"
)

func (c *Config) ToWindowsExporterConfig() collector.Config {
	cfg := collector.ConfigDefaults
	cfg.Dfsr.DfsrEnabledCollectors = c.Dfsr.SourcesEnabled
	cfg.Exchange.CollectorsEnabled = c.Exchange.EnabledList

	cfg.Iis.SiteInclude = coalesceString(c.IIS.SiteInclude, c.IIS.SiteWhiteList)
	cfg.Iis.SiteExclude = coalesceString(c.IIS.SiteExclude, c.IIS.SiteBlackList)
	cfg.Iis.AppInclude = coalesceString(c.IIS.AppInclude, c.IIS.AppWhiteList)
	cfg.Iis.AppExclude = coalesceString(c.IIS.AppExclude, c.IIS.AppBlackList)

	cfg.Service.ServiceWhereClause = c.Service.Where
	cfg.Service.UseAPI = c.Service.UseApi == "true"

	cfg.Smtp.ServerInclude = coalesceString(c.SMTP.Include, c.SMTP.WhiteList)
	cfg.Smtp.ServerExclude = coalesceString(c.SMTP.Exclude, c.SMTP.BlackList)

	cfg.Textfile.TextFileDirectories = c.TextFile.TextFileDirectory

	cfg.PhysicalDisk.DiskInclude = c.PhysicalDisk.Include
	cfg.PhysicalDisk.DiskExclude = c.PhysicalDisk.Exclude

	cfg.Process.ProcessExclude = coalesceString(c.Process.Exclude, c.Process.BlackList)
	cfg.Process.ProcessInclude = coalesceString(c.Process.Include, c.Process.WhiteList)

	cfg.Net.NicExclude = coalesceString(c.Network.Exclude, c.Network.BlackList)
	cfg.Net.NicInclude = coalesceString(c.Network.Include, c.Network.WhiteList)

	cfg.Mssql.EnabledCollectors = c.MSSQL.EnabledClasses

	cfg.Msmq.QueryWhereClause = c.MSMQ.Where

	cfg.LogicalDisk.VolumeInclude = coalesceString(c.LogicalDisk.Include, c.LogicalDisk.WhiteList)
	cfg.LogicalDisk.VolumeExclude = coalesceString(c.LogicalDisk.Exclude, c.LogicalDisk.BlackList)

	cfg.ScheduledTask.TaskInclude = c.ScheduledTask.Include
	cfg.ScheduledTask.TaskExclude = c.ScheduledTask.Exclude

	return cfg
}

func coalesceString(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// DefaultConfig holds the default settings for the windows_exporter integration.
var DefaultConfig = Config{
	EnabledCollectors: "cpu,cs,logical_disk,net,os,service,system",
	Dfsr: DfsrConfig{
		SourcesEnabled: collector.ConfigDefaults.Dfsr.DfsrEnabledCollectors,
	},
	Exchange: ExchangeConfig{
		EnabledList: collector.ConfigDefaults.Exchange.CollectorsEnabled,
	},
	IIS: IISConfig{
		AppBlackList:  collector.ConfigDefaults.Iis.AppExclude,
		AppWhiteList:  collector.ConfigDefaults.Iis.AppInclude,
		SiteBlackList: collector.ConfigDefaults.Iis.SiteExclude,
		SiteWhiteList: collector.ConfigDefaults.Iis.SiteInclude,
		AppInclude:    collector.ConfigDefaults.Iis.AppInclude,
		AppExclude:    collector.ConfigDefaults.Iis.AppExclude,
		SiteInclude:   collector.ConfigDefaults.Iis.SiteInclude,
		SiteExclude:   collector.ConfigDefaults.Iis.SiteExclude,
	},
	LogicalDisk: LogicalDiskConfig{
		BlackList: collector.ConfigDefaults.LogicalDisk.VolumeExclude,
		WhiteList: collector.ConfigDefaults.LogicalDisk.VolumeInclude,
		Include:   collector.ConfigDefaults.LogicalDisk.VolumeInclude,
		Exclude:   collector.ConfigDefaults.LogicalDisk.VolumeExclude,
	},
	MSMQ: MSMQConfig{
		Where: collector.ConfigDefaults.Msmq.QueryWhereClause,
	},
	MSSQL: MSSQLConfig{
		EnabledClasses: collector.ConfigDefaults.Mssql.EnabledCollectors,
	},
	Network: NetworkConfig{
		BlackList: collector.ConfigDefaults.Net.NicExclude,
		WhiteList: collector.ConfigDefaults.Net.NicInclude,
		Include:   collector.ConfigDefaults.Net.NicInclude,
		Exclude:   collector.ConfigDefaults.Net.NicExclude,
	},
	PhysicalDisk: PhysicalDiskConfig{
		Include: collector.ConfigDefaults.PhysicalDisk.DiskInclude,
		Exclude: collector.ConfigDefaults.PhysicalDisk.DiskExclude,
	},
	Process: ProcessConfig{
		BlackList: collector.ConfigDefaults.Process.ProcessExclude,
		WhiteList: collector.ConfigDefaults.Process.ProcessInclude,
		Include:   collector.ConfigDefaults.Process.ProcessInclude,
		Exclude:   collector.ConfigDefaults.Process.ProcessExclude,
	},
	ScheduledTask: ScheduledTaskConfig{
		Include: collector.ConfigDefaults.ScheduledTask.TaskInclude,
		Exclude: collector.ConfigDefaults.ScheduledTask.TaskExclude,
	},
	Service: ServiceConfig{
		UseApi: "false",
		Where:  collector.ConfigDefaults.Service.ServiceWhereClause,
	},
	SMTP: SMTPConfig{
		BlackList: collector.ConfigDefaults.Smtp.ServerExclude,
		WhiteList: collector.ConfigDefaults.Smtp.ServerInclude,
		Include:   collector.ConfigDefaults.Smtp.ServerInclude,
		Exclude:   collector.ConfigDefaults.Smtp.ServerExclude,
	},
	TextFile: TextFileConfig{
		TextFileDirectory: collector.ConfigDefaults.Textfile.TextFileDirectories,
	},
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}
