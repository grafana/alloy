package windows_exporter

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus-community/windows_exporter/pkg/collector"
)

func (c *Config) ToWindowsExporterConfig() (collector.Config, error) {
	var err error

	errs := make([]error, 0, 18)

	cfg := collector.ConfigDefaults
	cfg.DFSR.CollectorsEnabled = strings.Split(c.Dfsr.SourcesEnabled, ",")
	cfg.Exchange.CollectorsEnabled = strings.Split(c.Exchange.EnabledList, ",")

	cfg.IIS.SiteInclude, err = regexp.Compile(coalesceString(c.IIS.SiteInclude, c.IIS.SiteWhiteList))
	errs = append(errs, err)
	cfg.IIS.SiteExclude, err = regexp.Compile(coalesceString(c.IIS.SiteExclude, c.IIS.SiteBlackList))
	errs = append(errs, err)
	cfg.IIS.AppInclude, err = regexp.Compile(coalesceString(c.IIS.AppInclude, c.IIS.AppWhiteList))
	errs = append(errs, err)
	cfg.IIS.AppExclude, err = regexp.Compile(coalesceString(c.IIS.AppExclude, c.IIS.AppBlackList))
	errs = append(errs, err)

	cfg.Service.ServiceWhereClause = c.Service.Where
	cfg.Service.UseAPI = c.Service.UseApi == "true"
	cfg.Service.V2 = c.Service.V2 == "true"

	cfg.SMTP.ServerInclude, err = regexp.Compile(coalesceString(c.SMTP.Include, c.SMTP.WhiteList))
	errs = append(errs, err)
	cfg.SMTP.ServerExclude, err = regexp.Compile(coalesceString(c.SMTP.Exclude, c.SMTP.BlackList))
	errs = append(errs, err)

	cfg.Textfile.TextFileDirectories = strings.Split(c.TextFile.TextFileDirectory, ",")

	cfg.PhysicalDisk.DiskInclude, err = regexp.Compile(c.PhysicalDisk.Include)
	errs = append(errs, err)
	cfg.PhysicalDisk.DiskExclude, err = regexp.Compile(c.PhysicalDisk.Exclude)
	errs = append(errs, err)

	cfg.Printer.PrinterInclude, err = regexp.Compile(c.Printer.Include)
	errs = append(errs, err)
	cfg.Printer.PrinterExclude, err = regexp.Compile(c.Printer.Exclude)
	errs = append(errs, err)

	cfg.Process.ProcessExclude, err = regexp.Compile(coalesceString(c.Process.Exclude, c.Process.BlackList))
	errs = append(errs, err)
	cfg.Process.ProcessInclude, err = regexp.Compile(coalesceString(c.Process.Include, c.Process.WhiteList))
	errs = append(errs, err)

	cfg.Net.NicExclude, err = regexp.Compile(coalesceString(c.Network.Exclude, c.Network.BlackList))
	errs = append(errs, err)
	cfg.Net.NicInclude, err = regexp.Compile(coalesceString(c.Network.Include, c.Network.WhiteList))
	errs = append(errs, err)

	cfg.Mssql.CollectorsEnabled = strings.Split(c.MSSQL.EnabledClasses, ",")

	cfg.Msmq.QueryWhereClause = &c.MSMQ.Where

	cfg.LogicalDisk.VolumeInclude, err = regexp.Compile(coalesceString(c.LogicalDisk.Include, c.LogicalDisk.WhiteList))
	errs = append(errs, err)
	cfg.LogicalDisk.VolumeExclude, err = regexp.Compile(coalesceString(c.LogicalDisk.Exclude, c.LogicalDisk.BlackList))
	errs = append(errs, err)

	cfg.ScheduledTask.TaskInclude, err = regexp.Compile(c.ScheduledTask.Include)
	errs = append(errs, err)
	cfg.ScheduledTask.TaskExclude, err = regexp.Compile(c.ScheduledTask.Exclude)
	errs = append(errs, err)

	return cfg, errors.Join(errs...)
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
