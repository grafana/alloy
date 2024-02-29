package windows

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

var (
	exampleRiverConfig = `
		enabled_collectors = ["textfile","cpu"]
		
		exchange {
			enabled_list = ["example"]
		}
		
		iis {
			site_include = ".+"
			site_exclude = ""
			app_include = ".+"
			app_exclude = ""
		}
		
		text_file {
			text_file_directory = "C:"
		}
		
		smtp {
			include = ".+"
			exclude = ""
		}

        service {
            where_clause = "where"
        }

		physical_disk {
			include = ".+"
			exclude = ""
		}
		
		process {
			include = ".+"
			exclude = ""
		}
		
		network {
			include = ".+"
			exclude = ""
		}
		
		mssql {
			enabled_classes = ["accessmethods"]
		}
		
		msmq {
            where_clause = "where"
		}
		
		logical_disk {
			include = ".+"
			exclude = ""
		}
		`
)

func TestRiverUnmarshal(t *testing.T) {
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	require.Equal(t, []string{"textfile", "cpu"}, args.EnabledCollectors)
	require.Equal(t, []string{"example"}, args.Exchange.EnabledList)
	require.Equal(t, "", args.IIS.SiteExclude)
	require.Equal(t, ".+", args.IIS.SiteInclude)
	require.Equal(t, "", args.IIS.AppExclude)
	require.Equal(t, ".+", args.IIS.AppInclude)
	require.Equal(t, "C:", args.TextFile.TextFileDirectory)
	require.Equal(t, "", args.SMTP.Exclude)
	require.Equal(t, ".+", args.SMTP.Include)
	require.Equal(t, "where", args.Service.Where)
	require.Equal(t, "", args.PhysicalDisk.Exclude)
	require.Equal(t, ".+", args.PhysicalDisk.Include)
	require.Equal(t, "", args.Process.Exclude)
	require.Equal(t, ".+", args.Process.Include)
	require.Equal(t, "", args.Network.Exclude)
	require.Equal(t, ".+", args.Network.Include)
	require.Equal(t, []string{"accessmethods"}, args.MSSQL.EnabledClasses)
	require.Equal(t, "where", args.MSMQ.Where)
	require.Equal(t, "", args.LogicalDisk.Exclude)
	require.Equal(t, ".+", args.LogicalDisk.Include)
}

func TestConvert(t *testing.T) {
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	conf := args.Convert()

	require.Equal(t, "textfile,cpu", conf.EnabledCollectors)
	require.Equal(t, "example", conf.Exchange.EnabledList)
	require.Equal(t, "", conf.IIS.SiteExclude)
	require.Equal(t, ".+", conf.IIS.SiteInclude)
	require.Equal(t, "", conf.IIS.AppExclude)
	require.Equal(t, ".+", conf.IIS.AppInclude)
	require.Equal(t, "C:", conf.TextFile.TextFileDirectory)
	require.Equal(t, "", conf.SMTP.Exclude)
	require.Equal(t, ".+", conf.SMTP.Include)
	require.Equal(t, "where", conf.Service.Where)
	require.Equal(t, "", conf.PhysicalDisk.Exclude)
	require.Equal(t, ".+", conf.PhysicalDisk.Include)
	require.Equal(t, "", conf.Process.Exclude)
	require.Equal(t, ".+", conf.Process.Include)
	require.Equal(t, "", conf.Network.Exclude)
	require.Equal(t, ".+", conf.Network.Include)
	require.Equal(t, "accessmethods", conf.MSSQL.EnabledClasses)
	require.Equal(t, "where", conf.MSMQ.Where)
	require.Equal(t, "", conf.LogicalDisk.Exclude)
	require.Equal(t, ".+", conf.LogicalDisk.Include)
}
