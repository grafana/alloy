package build

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/windows"
	"github.com/grafana/alloy/internal/static/integrations/windows_exporter"
)

func (b *ConfigBuilder) appendWindowsExporter(config *windows_exporter.Config, instanceKey *string) discovery.Exports {
	args := toWindowsExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "windows")
}

// Splits a string such as "example1,example2"
// into a list such as []string{"example1", "example2"}.
func split(collectorList string) []string {
	if collectorList == "" {
		return []string{}
	}
	return strings.Split(collectorList, ",")
}

// This behavior is copied from getDefaultPath() function in:
// windows_exporter@v0.27.4-0.20241010144849-a0f6d3bcf9a4\pkg\collector\textfile\textfile.go
func getDefaultTextFileConfig() windows_exporter.TextFileConfig {
	execPath, _ := os.Executable()
	return windows_exporter.TextFileConfig{
		TextFileDirectory: filepath.Join(filepath.Dir(execPath), "textfile_inputs"),
	}
}

func toWindowsExporter(config *windows_exporter.Config) *windows.Arguments {
	// In order to support the rename of text_file block to textfile,
	// we need to leave them both as nil in the Arguments default, but that breaks
	// the behavior of the syntax token builder encodeFields function checking for default values
	var textfile *windows.TextFileConfig
	if !reflect.DeepEqual(config.TextFile, getDefaultTextFileConfig()) {
		textfile = &windows.TextFileConfig{
			Directories: split(config.TextFile.TextFileDirectory),
		}
	}

	return &windows.Arguments{
		EnabledCollectors: split(config.EnabledCollectors),
		Dfsr: windows.DfsrConfig{
			SourcesEnabled: split(config.Dfsr.SourcesEnabled),
		},
		Exchange: windows.ExchangeConfig{
			EnabledList: split(config.Exchange.EnabledList),
		},
		Filetime: windows.FiletimeConfig{
			FilePatterns: config.Filetime.FilePatterns,
		},
		IIS: windows.IISConfig{
			AppBlackList:  config.IIS.AppBlackList,
			AppWhiteList:  config.IIS.AppWhiteList,
			SiteBlackList: config.IIS.SiteBlackList,
			SiteWhiteList: config.IIS.SiteWhiteList,
			AppExclude:    config.IIS.AppExclude,
			AppInclude:    config.IIS.AppInclude,
			SiteExclude:   config.IIS.SiteExclude,
			SiteInclude:   config.IIS.SiteInclude,
		},
		LogicalDisk: windows.LogicalDiskConfig{
			BlackList:   config.LogicalDisk.BlackList,
			WhiteList:   config.LogicalDisk.WhiteList,
			Include:     config.LogicalDisk.Include,
			Exclude:     config.LogicalDisk.Exclude,
			EnabledList: split(config.LogicalDisk.EnabledList),
		},
		MSSQL: windows.MSSQLConfig{
			EnabledClasses: split(config.MSSQL.EnabledClasses),
		},
		MSCluster: windows.MSClusterConfig{
			EnabledList: split(config.MSCluster.EnabledList),
		},
		Network: windows.NetworkConfig{
			BlackList: config.Network.BlackList,
			WhiteList: config.Network.WhiteList,
			Exclude:   config.Network.Exclude,
			Include:   config.Network.Include,
		},
		NetFramework: windows.NetFrameworkConfig{
			EnabledList: split(config.NetFramework.EnabledList),
		},
		PerformanceCounter: windows.PerformanceCounterConfig{
			Objects: config.PerformanceCounter.Objects,
		},
		PhysicalDisk: windows.PhysicalDiskConfig{
			Exclude: config.PhysicalDisk.Exclude,
			Include: config.PhysicalDisk.Include,
		},
		Printer: windows.PrinterConfig{
			Exclude: config.Printer.Exclude,
			Include: config.Printer.Include,
		},
		Process: windows.ProcessConfig{
			BlackList:              config.Process.BlackList,
			WhiteList:              config.Process.WhiteList,
			Exclude:                config.Process.Exclude,
			Include:                config.Process.Include,
			EnableIISWorkerProcess: config.Process.EnableIISWorkerProcess,
			CounterVersion:         config.Process.CounterVersion,
		},
		ScheduledTask: windows.ScheduledTaskConfig{
			Exclude: config.ScheduledTask.Exclude,
			Include: config.ScheduledTask.Include,
		},
		Service: windows.ServiceConfig{
			Include: config.Service.Include,
			Exclude: config.Service.Exclude,
		},
		SMB: windows.SMBConfig{
			EnabledList: split(config.SMB.EnabledList),
		},
		SMBClient: windows.SMBClientConfig{
			EnabledList: split(config.SMBClient.EnabledList),
		},
		SMTP: windows.SMTPConfig{
			BlackList: config.SMTP.BlackList,
			WhiteList: config.SMTP.WhiteList,
			Exclude:   config.SMTP.Exclude,
			Include:   config.SMTP.Include,
		},
		TextFile: textfile,
		TCP: windows.TCPConfig{
			EnabledList: split(config.TCP.EnabledList),
		},
		DNS: windows.DNSConfig{
			EnabledList: split(config.DNS.EnabledList),
		},
		Update: windows.UpdateConfig{
			Online:         config.Update.Online,
			ScrapeInterval: config.Update.ScrapeInterval,
		},
		Net: windows.NetConfig{
			Include:     config.Net.Include,
			Exclude:     config.Net.Exclude,
			EnabledList: split(config.Net.EnabledList),
		},
	}
}
