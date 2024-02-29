package windows

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshalWithDefaultConfig(t *testing.T) {
	var args Arguments
	err := river.Unmarshal([]byte(""), &args)
	require.NoError(t, err)

	require.Equal(t, DefaultArguments.EnabledCollectors, args.EnabledCollectors)
	require.Equal(t, DefaultArguments.Dfsr.SourcesEnabled, args.Dfsr.SourcesEnabled)
	require.Equal(t, DefaultArguments.Exchange.EnabledList, args.Exchange.EnabledList)
	require.Equal(t, DefaultArguments.IIS.AppExclude, args.IIS.AppExclude)
	require.Equal(t, DefaultArguments.IIS.AppInclude, args.IIS.AppInclude)
	require.Equal(t, DefaultArguments.IIS.SiteExclude, args.IIS.SiteExclude)
	require.Equal(t, DefaultArguments.IIS.SiteInclude, args.IIS.SiteInclude)
	require.Equal(t, DefaultArguments.LogicalDisk.Exclude, args.LogicalDisk.Exclude)
	require.Equal(t, DefaultArguments.LogicalDisk.Include, args.LogicalDisk.Include)
	require.Equal(t, DefaultArguments.MSMQ.Where, args.MSMQ.Where)
	require.Equal(t, DefaultArguments.MSSQL.EnabledClasses, args.MSSQL.EnabledClasses)
	require.Equal(t, DefaultArguments.Network.Exclude, args.Network.Exclude)
	require.Equal(t, DefaultArguments.Network.Include, args.Network.Include)
	require.Equal(t, DefaultArguments.PhysicalDisk.Exclude, args.PhysicalDisk.Exclude)
	require.Equal(t, DefaultArguments.PhysicalDisk.Include, args.PhysicalDisk.Include)
	require.Equal(t, DefaultArguments.Process.Exclude, args.Process.Exclude)
	require.Equal(t, DefaultArguments.Process.Include, args.Process.Include)
	require.Equal(t, DefaultArguments.ScheduledTask.Exclude, args.ScheduledTask.Exclude)
	require.Equal(t, DefaultArguments.ScheduledTask.Include, args.ScheduledTask.Include)
	require.Equal(t, DefaultArguments.Service.UseApi, args.Service.UseApi)
	require.Equal(t, DefaultArguments.Service.Where, args.Service.Where)
	require.Equal(t, DefaultArguments.SMTP.Exclude, args.SMTP.Exclude)
	require.Equal(t, DefaultArguments.SMTP.Include, args.SMTP.Include)
	require.Equal(t, DefaultArguments.TextFile.TextFileDirectory, args.TextFile.TextFileDirectory)
}
