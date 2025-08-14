package windows

import (
	"slices"
	"strings"

	windows_integration "github.com/grafana/alloy/internal/static/integrations/windows_exporter"
	col "github.com/prometheus-community/windows_exporter/pkg/collector"
)

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		EnabledCollectors: strings.Split(windows_integration.DefaultConfig.EnabledCollectors, ","),
		Dfsr: DfsrConfig{
			SourcesEnabled: slices.Clone(col.ConfigDefaults.DFSR.CollectorsEnabled),
		},
		Exchange: ExchangeConfig{
			EnabledList: slices.Clone(col.ConfigDefaults.Exchange.CollectorsEnabled),
		},
		IIS: IISConfig{
			AppBlackList:  col.ConfigDefaults.IIS.AppExclude.String(),
			AppWhiteList:  col.ConfigDefaults.IIS.AppInclude.String(),
			SiteBlackList: col.ConfigDefaults.IIS.SiteExclude.String(),
			SiteWhiteList: col.ConfigDefaults.IIS.SiteInclude.String(),
			AppInclude:    col.ConfigDefaults.IIS.AppInclude.String(),
			AppExclude:    col.ConfigDefaults.IIS.AppExclude.String(),
			SiteInclude:   col.ConfigDefaults.IIS.SiteInclude.String(),
			SiteExclude:   col.ConfigDefaults.IIS.SiteExclude.String(),
		},
		LogicalDisk: LogicalDiskConfig{
			BlackList:   col.ConfigDefaults.LogicalDisk.VolumeExclude.String(),
			WhiteList:   col.ConfigDefaults.LogicalDisk.VolumeInclude.String(),
			Include:     col.ConfigDefaults.LogicalDisk.VolumeInclude.String(),
			Exclude:     col.ConfigDefaults.LogicalDisk.VolumeExclude.String(),
			EnabledList: slices.Clone(col.ConfigDefaults.LogicalDisk.CollectorsEnabled),
		},
		MSSQL: MSSQLConfig{
			EnabledClasses: slices.Clone(col.ConfigDefaults.Mssql.CollectorsEnabled),
		},
		Network: NetworkConfig{
			BlackList: col.ConfigDefaults.Net.NicExclude.String(),
			WhiteList: col.ConfigDefaults.Net.NicInclude.String(),
			Include:   col.ConfigDefaults.Net.NicInclude.String(),
			Exclude:   col.ConfigDefaults.Net.NicExclude.String(),
		},
		PhysicalDisk: PhysicalDiskConfig{
			Exclude: col.ConfigDefaults.PhysicalDisk.DiskExclude.String(),
			Include: col.ConfigDefaults.PhysicalDisk.DiskInclude.String(),
		},
		Printer: PrinterConfig{
			Include: col.ConfigDefaults.Printer.PrinterInclude.String(),
			Exclude: col.ConfigDefaults.Printer.PrinterExclude.String(),
		},
		Process: ProcessConfig{
			BlackList:              col.ConfigDefaults.Process.ProcessExclude.String(),
			WhiteList:              col.ConfigDefaults.Process.ProcessInclude.String(),
			Include:                col.ConfigDefaults.Process.ProcessInclude.String(),
			Exclude:                col.ConfigDefaults.Process.ProcessExclude.String(),
			EnableIISWorkerProcess: col.ConfigDefaults.Process.EnableWorkerProcess,
			CounterVersion:         col.ConfigDefaults.Process.CounterVersion,
		},
		ScheduledTask: ScheduledTaskConfig{
			Include: col.ConfigDefaults.ScheduledTask.TaskInclude.String(),
			Exclude: col.ConfigDefaults.ScheduledTask.TaskExclude.String(),
		},
		Service: ServiceConfig{
			Include: col.ConfigDefaults.Service.ServiceInclude.String(),
			Exclude: col.ConfigDefaults.Service.ServiceExclude.String(),
		},
		SMB: SMBConfig{
			EnabledList: []string{},
		},
		SMBClient: SMBClientConfig{
			EnabledList: []string{},
		},
		SMTP: SMTPConfig{
			BlackList: col.ConfigDefaults.SMTP.ServerExclude.String(),
			WhiteList: col.ConfigDefaults.SMTP.ServerInclude.String(),
			Include:   col.ConfigDefaults.SMTP.ServerInclude.String(),
			Exclude:   col.ConfigDefaults.SMTP.ServerExclude.String(),
		},
		// Keep the defaults in the deprecated field & block for backward compatibility.
		// TextFileDeprecated & TextFile are both pointer types to allow for identification
		// of whether the user has set the field or not, so we don't initialize them here.
		TCP: TCPConfig{
			EnabledList: slices.Clone(col.ConfigDefaults.TCP.CollectorsEnabled),
		},
		Filetime: FiletimeConfig{
			FilePatterns: slices.Clone(col.ConfigDefaults.Filetime.FilePatterns),
		},
		MSCluster: MSClusterConfig{
			EnabledList: slices.Clone(col.ConfigDefaults.MSCluster.CollectorsEnabled),
		},
		NetFramework: NetFrameworkConfig{
			EnabledList: slices.Clone(col.ConfigDefaults.NetFramework.CollectorsEnabled),
		},
		DNS: DNSConfig{
			EnabledList: slices.Clone(col.ConfigDefaults.DNS.CollectorsEnabled),
		},
		Update: UpdateConfig{
			Online:         col.ConfigDefaults.Update.Online,
			ScrapeInterval: col.ConfigDefaults.Update.ScrapeInterval,
		},
		Net: NetConfig{
			EnabledList: slices.Clone(col.ConfigDefaults.Net.CollectorsEnabled),
			Exclude:     col.ConfigDefaults.Net.NicExclude.String(),
			Include:     col.ConfigDefaults.Net.NicInclude.String(),
		},
	}
}
