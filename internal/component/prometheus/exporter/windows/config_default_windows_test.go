package windows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshalWithDefaultConfig(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(""), &args)
	require.NoError(t, err)

	var defaultArgs Arguments
	defaultArgs.SetToDefault()
	require.Equal(t, defaultArgs.EnabledCollectors, args.EnabledCollectors)
	require.Equal(t, defaultArgs.Dfsr.SourcesEnabled, args.Dfsr.SourcesEnabled)
	require.Equal(t, defaultArgs.Exchange.EnabledList, args.Exchange.EnabledList)
	require.Equal(t, defaultArgs.IIS.AppExclude, args.IIS.AppExclude)
	require.Equal(t, defaultArgs.IIS.AppInclude, args.IIS.AppInclude)
	require.Equal(t, defaultArgs.IIS.SiteExclude, args.IIS.SiteExclude)
	require.Equal(t, defaultArgs.IIS.SiteInclude, args.IIS.SiteInclude)
	require.Equal(t, defaultArgs.LogicalDisk.Exclude, args.LogicalDisk.Exclude)
	require.Equal(t, defaultArgs.LogicalDisk.Include, args.LogicalDisk.Include)
	require.Equal(t, defaultArgs.MSMQ.Where, args.MSMQ.Where)
	require.Equal(t, defaultArgs.MSSQL.EnabledClasses, args.MSSQL.EnabledClasses)
	require.Equal(t, defaultArgs.Network.Exclude, args.Network.Exclude)
	require.Equal(t, defaultArgs.Network.Include, args.Network.Include)
	require.Equal(t, defaultArgs.PhysicalDisk.Exclude, args.PhysicalDisk.Exclude)
	require.Equal(t, defaultArgs.PhysicalDisk.Include, args.PhysicalDisk.Include)
	require.Equal(t, defaultArgs.Process.Exclude, args.Process.Exclude)
	require.Equal(t, defaultArgs.Process.Include, args.Process.Include)
	require.Equal(t, defaultArgs.ScheduledTask.Exclude, args.ScheduledTask.Exclude)
	require.Equal(t, defaultArgs.ScheduledTask.Include, args.ScheduledTask.Include)
	require.Equal(t, defaultArgs.Service.UseApi, args.Service.UseApi)
	require.Equal(t, defaultArgs.Service.Where, args.Service.Where)
	require.Equal(t, defaultArgs.Service.V2, args.Service.V2)
	require.Equal(t, defaultArgs.Printer.Exclude, args.Printer.Exclude)
	require.Equal(t, defaultArgs.Printer.Include, args.Printer.Include)
	require.Equal(t, defaultArgs.SMB.EnabledList, args.SMB.EnabledList)
	require.Equal(t, defaultArgs.SMBClient.EnabledList, args.SMBClient.EnabledList)
	require.Equal(t, defaultArgs.SMTP.Exclude, args.SMTP.Exclude)
	require.Equal(t, defaultArgs.SMTP.Include, args.SMTP.Include)
	require.Equal(t, defaultArgs.TextFile.TextFileDirectory, args.TextFile.TextFileDirectory)
}

// This is a copy of the getDefaultPath() function in:
// windows_exporter@v0.27.4-0.20241010144849-a0f6d3bcf9a4\pkg\collector\textfile\textfile.go
func getDefaultTextFilePath() string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), "textfile_inputs")
}

func TestDefaultConfig(t *testing.T) {
	// TODO: The BlackList and WhiteList attributes should be removed in Alloy v2.
	// They are not even documented in Alloy v1.
	expected := Arguments{
		EnabledCollectors: []string{"cpu", "cs", "logical_disk", "net", "os", "service", "system"},
		Dfsr:              DfsrConfig{SourcesEnabled: []string{"connection", "folder", "volume"}},
		Exchange:          ExchangeConfig{EnabledList: []string{"ADAccessProcesses", "TransportQueues", "HttpProxy", "ActiveSync", "AvailabilityService", "OutlookWebAccess", "Autodiscover", "WorkloadManagement", "RpcClientAccess", "MapiHttpEmsmdb"}},
		IIS:               IISConfig{AppBlackList: "^$", AppWhiteList: "^.+$", SiteBlackList: "^$", SiteWhiteList: "^.+$", AppExclude: "^$", AppInclude: "^.+$", SiteExclude: "^$", SiteInclude: "^.+$"},
		LogicalDisk:       LogicalDiskConfig{BlackList: "^$", WhiteList: "^.+$", Include: "^.+$", Exclude: "^$"},
		MSMQ:              MSMQConfig{Where: ""},
		MSSQL:             MSSQLConfig{EnabledClasses: []string{"accessmethods", "availreplica", "bufman", "databases", "dbreplica", "genstats", "locks", "memmgr", "sqlstats", "sqlerrors", "transactions", "waitstats"}},
		Network:           NetworkConfig{BlackList: "^$", WhiteList: "^.+$", Exclude: "^$", Include: "^.+$"},
		PhysicalDisk:      PhysicalDiskConfig{Include: "^.+$", Exclude: "^$"},
		Printer:           PrinterConfig{Exclude: "^$", Include: "^.+$"},
		Process:           ProcessConfig{BlackList: "^$", WhiteList: "^.+$", Exclude: "^$", Include: "^.+$"},
		ScheduledTask:     ScheduledTaskConfig{Exclude: "^$", Include: "^.+$"},
		Service:           ServiceConfig{UseApi: "false", Where: "", V2: "false"},
		SMB:               SMBConfig{EnabledList: []string{}},
		SMBClient:         SMBClientConfig{EnabledList: []string{}},
		SMTP:              SMTPConfig{BlackList: "^$", WhiteList: "^.+$", Exclude: "^$", Include: "^.+$"},
		TextFile:          TextFileConfig{TextFileDirectory: getDefaultTextFilePath()},
	}

	var args Arguments
	err := syntax.Unmarshal([]byte(""), &args)
	require.NoError(t, err)
	require.Equal(t, expected, args)

	var defaultArgs Arguments
	defaultArgs.SetToDefault()
	require.Equal(t, expected, defaultArgs)
}
