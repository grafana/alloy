package windows_exporter

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus-community/windows_exporter/pkg/collector"
)

func (c *Config) ToWindowsExporterConfig() collector.Config {
	cfg := collector.ConfigDefaults
	cfg.DFSR.CollectorsEnabled = strings.Split(c.Dfsr.SourcesEnabled, ",")
	cfg.Exchange.CollectorsEnabled = strings.Split(c.Exchange.EnabledList, ",")

	cfg.IIS.SiteInclude = regexp.MustCompile(coalesceString(c.IIS.SiteInclude, c.IIS.SiteWhiteList))
	cfg.IIS.SiteExclude = regexp.MustCompile(coalesceString(c.IIS.SiteExclude, c.IIS.SiteBlackList))
	cfg.IIS.AppInclude = regexp.MustCompile(coalesceString(c.IIS.AppInclude, c.IIS.AppWhiteList))
	cfg.IIS.AppExclude = regexp.MustCompile(coalesceString(c.IIS.AppExclude, c.IIS.AppBlackList))

	cfg.Service.ServiceWhereClause = c.Service.Where
	cfg.Service.UseAPI = c.Service.UseApi == "true"
	cfg.Service.V2 = c.Service.V2 == "true"

	cfg.SMTP.ServerInclude = regexp.MustCompile(coalesceString(c.SMTP.Include, c.SMTP.WhiteList))
	cfg.SMTP.ServerExclude = regexp.MustCompile(coalesceString(c.SMTP.Exclude, c.SMTP.BlackList))

	cfg.Textfile.TextFileDirectories = strings.Split(c.TextFile.TextFileDirectory, ",")

	cfg.PhysicalDisk.DiskInclude = regexp.MustCompile(c.PhysicalDisk.Include)
	cfg.PhysicalDisk.DiskExclude = regexp.MustCompile(c.PhysicalDisk.Exclude)

	cfg.Printer.PrinterInclude = regexp.MustCompile(c.Printer.Include)
	cfg.Printer.PrinterExclude = regexp.MustCompile(c.Printer.Exclude)

	cfg.Process.ProcessExclude = regexp.MustCompile(coalesceString(c.Process.Exclude, c.Process.BlackList))
	cfg.Process.ProcessInclude = regexp.MustCompile(coalesceString(c.Process.Include, c.Process.WhiteList))

	cfg.Net.NicExclude = regexp.MustCompile(coalesceString(c.Network.Exclude, c.Network.BlackList))
	cfg.Net.NicInclude = regexp.MustCompile(coalesceString(c.Network.Include, c.Network.WhiteList))

	cfg.Mssql.CollectorsEnabled = strings.Split(c.MSSQL.EnabledClasses, ",")

	cfg.Msmq.QueryWhereClause = &c.MSMQ.Where

	cfg.LogicalDisk.VolumeInclude = regexp.MustCompile(coalesceString(c.LogicalDisk.Include, c.LogicalDisk.WhiteList))
	cfg.LogicalDisk.VolumeExclude = regexp.MustCompile(coalesceString(c.LogicalDisk.Exclude, c.LogicalDisk.BlackList))

	cfg.ScheduledTask.TaskInclude = regexp.MustCompile(c.ScheduledTask.Include)
	cfg.ScheduledTask.TaskExclude = regexp.MustCompile(c.ScheduledTask.Exclude)

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
		SourcesEnabled: strings.Join(collector.ConfigDefaults.DFSR.CollectorsEnabled, ","),
	},
	Exchange: ExchangeConfig{
		EnabledList: strings.Join(collector.ConfigDefaults.Exchange.CollectorsEnabled, ","),
	},
	IIS: IISConfig{
		AppBlackList:  collector.ConfigDefaults.IIS.AppExclude.String(),
		AppWhiteList:  collector.ConfigDefaults.IIS.AppInclude.String(),
		SiteBlackList: collector.ConfigDefaults.IIS.SiteExclude.String(),
		SiteWhiteList: collector.ConfigDefaults.IIS.SiteInclude.String(),
		AppInclude:    collector.ConfigDefaults.IIS.AppInclude.String(),
		AppExclude:    collector.ConfigDefaults.IIS.AppExclude.String(),
		SiteInclude:   collector.ConfigDefaults.IIS.SiteInclude.String(),
		SiteExclude:   collector.ConfigDefaults.IIS.SiteExclude.String(),
	},
	LogicalDisk: LogicalDiskConfig{
		BlackList: collector.ConfigDefaults.LogicalDisk.VolumeExclude.String(),
		WhiteList: collector.ConfigDefaults.LogicalDisk.VolumeInclude.String(),
		Include:   collector.ConfigDefaults.LogicalDisk.VolumeInclude.String(),
		Exclude:   collector.ConfigDefaults.LogicalDisk.VolumeExclude.String(),
	},
	MSMQ: MSMQConfig{
		Where: *collector.ConfigDefaults.Msmq.QueryWhereClause,
	},
	MSSQL: MSSQLConfig{
		EnabledClasses: strings.Join(collector.ConfigDefaults.Mssql.CollectorsEnabled, ","),
	},
	Network: NetworkConfig{
		BlackList: collector.ConfigDefaults.Net.NicExclude.String(),
		WhiteList: collector.ConfigDefaults.Net.NicInclude.String(),
		Include:   collector.ConfigDefaults.Net.NicInclude.String(),
		Exclude:   collector.ConfigDefaults.Net.NicExclude.String(),
	},
	PhysicalDisk: PhysicalDiskConfig{
		Include: collector.ConfigDefaults.PhysicalDisk.DiskInclude.String(),
		Exclude: collector.ConfigDefaults.PhysicalDisk.DiskExclude.String(),
	},
	Printer: PrinterConfig{
		Include: collector.ConfigDefaults.Printer.PrinterInclude.String(),
		Exclude: collector.ConfigDefaults.Printer.PrinterExclude.String(),
	},
	Process: ProcessConfig{
		BlackList: collector.ConfigDefaults.Process.ProcessExclude.String(),
		WhiteList: collector.ConfigDefaults.Process.ProcessInclude.String(),
		Include:   collector.ConfigDefaults.Process.ProcessInclude.String(),
		Exclude:   collector.ConfigDefaults.Process.ProcessExclude.String(),
	},
	ScheduledTask: ScheduledTaskConfig{
		Include: collector.ConfigDefaults.ScheduledTask.TaskInclude.String(),
		Exclude: collector.ConfigDefaults.ScheduledTask.TaskExclude.String(),
	},
	Service: ServiceConfig{
		UseApi: strconv.FormatBool(collector.ConfigDefaults.Service.UseAPI),
		Where:  collector.ConfigDefaults.Service.ServiceWhereClause,
		V2:     strconv.FormatBool(collector.ConfigDefaults.Service.V2),
	},
	SMTP: SMTPConfig{
		BlackList: collector.ConfigDefaults.SMTP.ServerExclude.String(),
		WhiteList: collector.ConfigDefaults.SMTP.ServerInclude.String(),
		Include:   collector.ConfigDefaults.SMTP.ServerInclude.String(),
		Exclude:   collector.ConfigDefaults.SMTP.ServerExclude.String(),
	},
	SMB: SMBConfig{
		EnabledList: "",
	},
	SMBClient: SMBClientConfig{
		EnabledList: "",
	},
	TextFile: TextFileConfig{
		TextFileDirectory: strings.Join(collector.ConfigDefaults.Textfile.TextFileDirectories, ","),
	},
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}
