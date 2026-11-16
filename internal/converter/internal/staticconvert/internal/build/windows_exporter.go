package build

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/windows"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/static/integrations/windows_exporter"
)

func (b *ConfigBuilder) appendWindowsExporter(config *windows_exporter.Config, instanceKey *string) discovery.Exports {
	if config.Process.CounterVersion != 0 {
		b.diags.Add(diag.SeverityLevelWarn, "The windows_exporter process counter_version option was removed in windows_exporter v0.32 and is not converted.")
	}
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

// translateCollectors renames filetime to file, drops thermalzone, and removes duplicates.
// windows_exporter v0.32 removed both of the old collector names.
func translateCollectors(collectors []string) []string {
	result := make([]string, 0, len(collectors))
	for _, name := range collectors {
		switch name {
		case "filetime":
			name = "file"
		case "thermalzone":
			continue
		}
		if !slices.Contains(result, name) {
			result = append(result, name)
		}
	}
	return result
}

// mergePatterns joins the file patterns without duplicates.
// It never returns nil, to match the default of the component.
func mergePatterns(lists ...[]string) []string {
	result := []string{}
	for _, list := range lists {
		for _, pattern := range list {
			if !slices.Contains(result, pattern) {
				result = append(result, pattern)
			}
		}
	}
	return result
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
		EnabledCollectors: translateCollectors(split(config.EnabledCollectors)),
		Dfsr: windows.DfsrConfig{
			SourcesEnabled: split(config.Dfsr.SourcesEnabled),
		},
		Exchange: windows.ExchangeConfig{
			EnabledList: split(config.Exchange.EnabledList),
		},
		File: windows.FileConfig{
			// The deprecated filetime patterns move to the file collector.
			FilePatterns: mergePatterns(config.File.FilePatterns, config.Filetime.FilePatterns),
		},
		Container: windows.ContainerConfig{
			CRIEndpoint: config.Container.CRIEndpoint,
		},
		Registry: windows.RegistryConfig{
			Keys: config.Registry.Keys,
		},
		WMI: windows.WMIConfig{
			Queries: config.WMI.Queries,
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
		},
		ScheduledTask: windows.ScheduledTaskConfig{
			Exclude:    config.ScheduledTask.Exclude,
			Include:    config.ScheduledTask.Include,
			MissedRuns: config.ScheduledTask.MissedRuns,
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
		Time: windows.TimeConfig{
			EnabledList: split(config.Time.EnabledList),
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
