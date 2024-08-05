package windows

import (
	"strconv"
	"strings"

	windows_integration "github.com/grafana/alloy/internal/static/integrations/windows_exporter"
	col "github.com/prometheus-community/windows_exporter/pkg/collector"
)

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		EnabledCollectors: strings.Split(windows_integration.DefaultConfig.EnabledCollectors, ","),
		Dfsr: DfsrConfig{
			SourcesEnabled: strings.Split(col.ConfigDefaults.Dfsr.DfsrEnabledCollectors, ","),
		},
		Exchange: ExchangeConfig{
			EnabledList: strings.Split(col.ConfigDefaults.Exchange.CollectorsEnabled, ","),
		},
		IIS: IISConfig{
			AppBlackList:  col.ConfigDefaults.Iis.AppExclude,
			AppWhiteList:  col.ConfigDefaults.Iis.AppInclude,
			SiteBlackList: col.ConfigDefaults.Iis.SiteExclude,
			SiteWhiteList: col.ConfigDefaults.Iis.SiteInclude,
			AppInclude:    col.ConfigDefaults.Iis.AppInclude,
			AppExclude:    col.ConfigDefaults.Iis.AppExclude,
			SiteInclude:   col.ConfigDefaults.Iis.SiteInclude,
			SiteExclude:   col.ConfigDefaults.Iis.SiteExclude,
		},
		LogicalDisk: LogicalDiskConfig{
			BlackList: col.ConfigDefaults.LogicalDisk.VolumeExclude,
			WhiteList: col.ConfigDefaults.LogicalDisk.VolumeInclude,
			Include:   col.ConfigDefaults.LogicalDisk.VolumeInclude,
			Exclude:   col.ConfigDefaults.LogicalDisk.VolumeExclude,
		},
		MSMQ: MSMQConfig{
			Where: col.ConfigDefaults.Msmq.QueryWhereClause,
		},
		MSSQL: MSSQLConfig{
			EnabledClasses: strings.Split(col.ConfigDefaults.Mssql.EnabledCollectors, ","),
		},
		Network: NetworkConfig{
			BlackList: col.ConfigDefaults.Net.NicExclude,
			WhiteList: col.ConfigDefaults.Net.NicInclude,
			Include:   col.ConfigDefaults.Net.NicInclude,
			Exclude:   col.ConfigDefaults.Net.NicExclude,
		},
		PhysicalDisk: PhysicalDiskConfig{
			Exclude: col.ConfigDefaults.PhysicalDisk.DiskExclude,
			Include: col.ConfigDefaults.PhysicalDisk.DiskInclude,
		},
		Printer: PrinterConfig{
			Include: col.ConfigDefaults.Printer.Include,
			Exclude: col.ConfigDefaults.Printer.Exclude,
		},
		Process: ProcessConfig{
			BlackList: col.ConfigDefaults.Process.ProcessExclude,
			WhiteList: col.ConfigDefaults.Process.ProcessInclude,
			Include:   col.ConfigDefaults.Process.ProcessInclude,
			Exclude:   col.ConfigDefaults.Process.ProcessExclude,
		},
		ScheduledTask: ScheduledTaskConfig{
			Include: col.ConfigDefaults.ScheduledTask.TaskInclude,
			Exclude: col.ConfigDefaults.ScheduledTask.TaskExclude,
		},
		Service: ServiceConfig{
			UseApi: strconv.FormatBool(col.ConfigDefaults.Service.UseAPI),
			Where:  col.ConfigDefaults.Service.ServiceWhereClause,
			V2:     strconv.FormatBool(col.ConfigDefaults.Service.V2),
		},
		SMB: SMBConfig{
			EnabledList: strings.Split(col.ConfigDefaults.Smb.CollectorsEnabled, ","),
		},
		SMBClient: SMBClientConfig{
			EnabledList: strings.Split(col.ConfigDefaults.SmbClient.CollectorsEnabled, ","),
		},
		SMTP: SMTPConfig{
			BlackList: col.ConfigDefaults.Smtp.ServerExclude,
			WhiteList: col.ConfigDefaults.Smtp.ServerInclude,
			Include:   col.ConfigDefaults.Smtp.ServerInclude,
			Exclude:   col.ConfigDefaults.Smtp.ServerExclude,
		},
		TextFile: TextFileConfig{
			TextFileDirectory: col.ConfigDefaults.Textfile.TextFileDirectories,
		},
	}
}
