package windows_exporter

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/prometheus-community/windows_exporter/pkg/collector"
	"gopkg.in/yaml.v3"
)

var netframeworkCollectors = map[string]struct{}{
	"netframework_clrexceptions":      {},
	"netframework_clrinterop":         {},
	"netframework_clrjit":             {},
	"netframework_clrloading":         {},
	"netframework_clrlocksandthreads": {},
	"netframework_clrmemory":          {},
	"netframework_clrremoting":        {},
	"netframework_clrsecurity":        {},
}

var msclusterCollectors = map[string]struct{}{
	"mscluster_cluster":       {},
	"mscluster_network":       {},
	"mscluster_node":          {},
	"mscluster_resource":      {},
	"mscluster_resourcegroup": {},
}

// normalizeCollectors translates the configured collector names to the names that
// windows_exporter accepts. It returns the top-level names and the sub-collectors
// for the grouped netframework and mscluster collectors. It does not change the config.
func normalizeCollectors(input string) (names []string, subCollectors map[string][]string) {
	enabled := map[string]struct{}{}
	subCollectors = map[string][]string{}
	for _, coll := range strings.Split(input, ",") {
		coll = strings.TrimSpace(coll)
		if coll == "" {
			continue
		}
		// windows_exporter v0.32 renamed filetime to file and removed thermalzone.
		// Stopping at startup would break existing configs, so translate or drop them.
		switch coll {
		case "filetime":
			coll = "file"
		case "thermalzone":
			continue
		}
		if _, ok := netframeworkCollectors[coll]; ok {
			enabled["netframework"] = struct{}{}
			subCollectors["netframework"] = append(subCollectors["netframework"], strings.TrimPrefix(coll, "netframework_"))
			continue
		}
		if _, ok := msclusterCollectors[coll]; ok {
			enabled["mscluster"] = struct{}{}
			subCollectors["mscluster"] = append(subCollectors["mscluster"], strings.TrimPrefix(coll, "mscluster_"))
			continue
		}
		enabled[coll] = struct{}{}
	}
	names = slices.Sorted(maps.Keys(enabled))
	return names, subCollectors
}

func (c *Config) ToWindowsExporterConfig() (collector.Config, error) {
	var err error

	_, enabledCollectors := normalizeCollectors(c.EnabledCollectors)

	errs := make([]error, 0, 18)

	cfg := collector.ConfigDefaults
	cfg.DFSR.CollectorsEnabled = splitNonEmpty(c.Dfsr.SourcesEnabled)
	cfg.Exchange.CollectorsEnabled = splitNonEmpty(c.Exchange.EnabledList)

	if len(c.MSCluster.EnabledList) > 0 || len(enabledCollectors["mscluster"]) > 0 {
		cfg.MSCluster.CollectorsEnabled = append(splitNonEmpty(c.MSCluster.EnabledList), enabledCollectors["mscluster"]...)
	}

	if len(c.NetFramework.EnabledList) > 0 || len(enabledCollectors["netframework"]) > 0 {
		cfg.NetFramework.CollectorsEnabled = append(splitNonEmpty(c.NetFramework.EnabledList), enabledCollectors["netframework"]...)
	}

	cfg.IIS.SiteInclude, err = regexp.Compile(coalesceString(c.IIS.SiteInclude, c.IIS.SiteWhiteList))
	errs = append(errs, err)
	cfg.IIS.SiteExclude, err = regexp.Compile(coalesceString(c.IIS.SiteExclude, c.IIS.SiteBlackList))
	errs = append(errs, err)
	cfg.IIS.AppInclude, err = regexp.Compile(coalesceString(c.IIS.AppInclude, c.IIS.AppWhiteList))
	errs = append(errs, err)
	cfg.IIS.AppExclude, err = regexp.Compile(coalesceString(c.IIS.AppExclude, c.IIS.AppBlackList))
	errs = append(errs, err)

	cfg.Service.ServiceExclude, err = regexp.Compile(c.Service.Exclude)
	errs = append(errs, err)
	cfg.Service.ServiceInclude, err = regexp.Compile(c.Service.Include)
	errs = append(errs, err)

	cfg.SMTP.ServerInclude, err = regexp.Compile(coalesceString(c.SMTP.Include, c.SMTP.WhiteList))
	errs = append(errs, err)
	cfg.SMTP.ServerExclude, err = regexp.Compile(coalesceString(c.SMTP.Exclude, c.SMTP.BlackList))
	errs = append(errs, err)

	if c.TextFile.TextFileDirectory != "" {
		cfg.Textfile.TextFileDirectories = strings.Split(c.TextFile.TextFileDirectory, ",")
	}

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
	cfg.Process.EnableWorkerProcess = c.Process.EnableIISWorkerProcess

	cfg.Net.NicExclude, err = regexp.Compile(coalesceString(c.Network.Exclude, c.Network.BlackList))
	errs = append(errs, err)
	cfg.Net.NicInclude, err = regexp.Compile(coalesceString(c.Network.Include, c.Network.WhiteList))
	errs = append(errs, err)

	cfg.Mssql.CollectorsEnabled = splitNonEmpty(c.MSSQL.EnabledClasses)

	cfg.LogicalDisk.VolumeInclude, err = regexp.Compile(coalesceString(c.LogicalDisk.Include, c.LogicalDisk.WhiteList))
	errs = append(errs, err)
	cfg.LogicalDisk.VolumeExclude, err = regexp.Compile(coalesceString(c.LogicalDisk.Exclude, c.LogicalDisk.BlackList))
	errs = append(errs, err)

	cfg.LogicalDisk.CollectorsEnabled = splitNonEmpty(c.LogicalDisk.EnabledList)

	cfg.ScheduledTask.TaskInclude, err = regexp.Compile(c.ScheduledTask.Include)
	errs = append(errs, err)
	cfg.ScheduledTask.TaskExclude, err = regexp.Compile(c.ScheduledTask.Exclude)
	errs = append(errs, err)

	// The deprecated filetime patterns use the same glob syntax as the file collector.
	filePatterns := slices.Concat(c.File.FilePatterns, c.Filetime.FilePatterns)
	slices.Sort(filePatterns)
	cfg.File.FilePatterns = slices.Compact(filePatterns)

	cfg.TCP.CollectorsEnabled = splitNonEmpty(c.TCP.EnabledList)

	// These objects are all internal and the best way to handle is to accept as a string and unmarshal
	// into the config struct
	if c.PerformanceCounter.Objects != "" {
		err = yaml.Unmarshal([]byte(c.PerformanceCounter.Objects), &cfg.PerformanceCounter.Objects)
		if err != nil {
			errs = append(errs, err)
		}
	}

	cfg.ScheduledTask.MissedRuns = c.ScheduledTask.MissedRuns

	if c.Container.CRIEndpoint != "" {
		cfg.Container.CRIEndpoint = c.Container.CRIEndpoint
	}

	if c.Registry.Keys != "" {
		err = decodeStrict(c.Registry.Keys, &cfg.Registry.Keys)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if c.WMI.Queries != "" {
		err = decodeStrict(c.WMI.Queries, &cfg.WMI.Queries)
		if err != nil {
			errs = append(errs, err)
		}
	}

	cfg.DNS.CollectorsEnabled = splitNonEmpty(c.DNS.EnabledList)

	cfg.Net.CollectorsEnabled = splitNonEmpty(c.Net.EnabledList)
	cfg.Net.NicExclude, err = regexp.Compile(c.Net.Exclude)
	errs = append(errs, err)
	cfg.Net.NicInclude, err = regexp.Compile(c.Net.Include)
	errs = append(errs, err)

	cfg.Time.CollectorsEnabled = splitNonEmpty(c.Time.EnabledList)

	return cfg, errors.Join(errs...)
}

// splitNonEmpty splits a comma-separated list and drops empty items.
// windows_exporter v0.32 rejects an empty sub-collector name.
func splitNonEmpty(list string) []string {
	items := make([]string, 0)
	for _, item := range strings.Split(list, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// decodeStrict decodes a YAML document and rejects unknown keys, like windows_exporter does.
func decodeStrict(doc string, out any) error {
	dec := yaml.NewDecoder(strings.NewReader(doc))
	dec.KnownFields(true)
	return dec.Decode(out)
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
	EnabledCollectors: "cpu,logical_disk,net,os,service,system",
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
		EnabledList: strings.Join(collector.ConfigDefaults.LogicalDisk.CollectorsEnabled, ","),
		BlackList:   collector.ConfigDefaults.LogicalDisk.VolumeExclude.String(),
		WhiteList:   collector.ConfigDefaults.LogicalDisk.VolumeInclude.String(),
		Include:     collector.ConfigDefaults.LogicalDisk.VolumeInclude.String(),
		Exclude:     collector.ConfigDefaults.LogicalDisk.VolumeExclude.String(),
	},
	MSSQL: MSSQLConfig{
		EnabledClasses: strings.Join(collector.ConfigDefaults.Mssql.CollectorsEnabled, ","),
	},
	MSCluster: MSClusterConfig{
		EnabledList: strings.Join(collector.ConfigDefaults.MSCluster.CollectorsEnabled, ","),
	},
	Network: NetworkConfig{
		BlackList: collector.ConfigDefaults.Net.NicExclude.String(),
		WhiteList: collector.ConfigDefaults.Net.NicInclude.String(),
		Include:   collector.ConfigDefaults.Net.NicInclude.String(),
		Exclude:   collector.ConfigDefaults.Net.NicExclude.String(),
	},
	NetFramework: NetFrameworkConfig{
		EnabledList: strings.Join(collector.ConfigDefaults.NetFramework.CollectorsEnabled, ","),
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
		BlackList:              collector.ConfigDefaults.Process.ProcessExclude.String(),
		WhiteList:              collector.ConfigDefaults.Process.ProcessInclude.String(),
		Include:                collector.ConfigDefaults.Process.ProcessInclude.String(),
		Exclude:                collector.ConfigDefaults.Process.ProcessExclude.String(),
		EnableIISWorkerProcess: collector.ConfigDefaults.Process.EnableWorkerProcess,
	},
	ScheduledTask: ScheduledTaskConfig{
		Include:    collector.ConfigDefaults.ScheduledTask.TaskInclude.String(),
		Exclude:    collector.ConfigDefaults.ScheduledTask.TaskExclude.String(),
		MissedRuns: collector.ConfigDefaults.ScheduledTask.MissedRuns,
	},
	Service: ServiceConfig{
		Include: collector.ConfigDefaults.Service.ServiceInclude.String(),
		Exclude: collector.ConfigDefaults.Service.ServiceExclude.String(),
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
	TCP: TCPConfig{
		EnabledList: strings.Join(collector.ConfigDefaults.TCP.CollectorsEnabled, ","),
	},
	Update: UpdateConfig{
		Online:         collector.ConfigDefaults.Update.Online,
		ScrapeInterval: collector.ConfigDefaults.Update.ScrapeInterval,
	},
	File: FileConfig{
		FilePatterns: collector.ConfigDefaults.File.FilePatterns,
	},

	PerformanceCounter: PerformanceCounterConfig{
		Objects: "", // default is empty, we yaml unmarshal the config
	},
	Container: ContainerConfig{
		CRIEndpoint: collector.ConfigDefaults.Container.CRIEndpoint,
	},
	DNS: DNSConfig{
		EnabledList: strings.Join(collector.ConfigDefaults.DNS.CollectorsEnabled, ","),
	},
	Net: NetConfig{
		EnabledList: strings.Join(collector.ConfigDefaults.Net.CollectorsEnabled, ","),
		Exclude:     collector.ConfigDefaults.Net.NicExclude.String(),
		Include:     collector.ConfigDefaults.Net.NicInclude.String(),
	},
	Time: TimeConfig{
		EnabledList: strings.Join(collector.ConfigDefaults.Time.CollectorsEnabled, ","),
	},
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}
