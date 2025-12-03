package windows

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

var (
	exampleAlloyConfig = `
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
            include = ".*"
			exclude = "alloy"
        }

		physical_disk {
			include = ".+"
			exclude = ""
		}

		printer {
			include = ".+"
			exclude = ""
		}

		process {
			include = ".+"
			exclude = ""
		}

		smb {
			enabled_list = ["example"]
		}

		smb_client {
			enabled_list = ["example"]
		}

		network {
			include = ".+"
			exclude = ""
		}

		mssql {
			enabled_classes = ["accessmethods"]
		}

		logical_disk {
			include = ".+"
			exclude = ""
		}

		tcp {
			enabled_list = ["example"]
		}

		filetime {
			file_patterns = ["example"]
		}

		performancecounter {
			objects =   "---" +
						"	- name: \"Processor\"" +
						"		counters:" +
						"			- name: \"% Processor Time\"" +
						"			  instance: \"_Total\"" +
						"			  type: \"counter\"" +
						"			  labels:" +
						"				  state: idle"

		}
		`
)

func TestAlloyUnmarshal(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	require.Equal(t, []string{"textfile", "cpu"}, args.EnabledCollectors)
	require.Equal(t, []string{"example"}, args.Exchange.EnabledList)
	require.Equal(t, "", args.IIS.SiteExclude)
	require.Equal(t, ".+", args.IIS.SiteInclude)
	require.Equal(t, "", args.IIS.AppExclude)
	require.Equal(t, ".+", args.IIS.AppInclude)
	require.Equal(t, "C:", args.TextFileDeprecated.TextFileDirectory)
	require.Equal(t, "", args.SMTP.Exclude)
	require.Equal(t, ".+", args.SMTP.Include)
	require.Equal(t, ".*", args.Service.Include)
	require.Equal(t, "alloy", args.Service.Exclude)
	require.Equal(t, "", args.PhysicalDisk.Exclude)
	require.Equal(t, ".+", args.PhysicalDisk.Include)
	require.Equal(t, "", args.Printer.Exclude)
	require.Equal(t, ".+", args.Printer.Include)
	require.Equal(t, []string{"example"}, args.SMB.EnabledList)
	require.Equal(t, []string{"example"}, args.SMBClient.EnabledList)
	require.Equal(t, "", args.Process.Exclude)
	require.Equal(t, ".+", args.Process.Include)
	require.Equal(t, "", args.Network.Exclude)
	require.Equal(t, ".+", args.Network.Include)
	require.Equal(t, []string{"accessmethods"}, args.MSSQL.EnabledClasses)
	require.Equal(t, "", args.LogicalDisk.Exclude)
	require.Equal(t, ".+", args.LogicalDisk.Include)

	require.Equal(t, []string{"example"}, args.TCP.EnabledList)
	require.Equal(t, []string{"example"}, args.Filetime.FilePatterns)
	// This isn't a real example, and the recommendation would be to use a file rather than a raw string
	require.Equal(t, "---\t- name: \"Processor\"\t\tcounters:\t\t\t- name: \"% Processor Time\"\t\t\t  instance: \"_Total\"\t\t\t  type: \"counter\"\t\t\t  labels:\t\t\t\t  state: idle", args.PerformanceCounter.Objects)
}

func TestConvert(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	conf := args.Convert(log.NewNopLogger())

	require.Equal(t, "textfile,cpu", conf.EnabledCollectors)
	require.Equal(t, "example", conf.Exchange.EnabledList)
	require.Equal(t, "^(?:)$", conf.IIS.SiteExclude)
	require.Equal(t, "^(?:.+)$", conf.IIS.SiteInclude)
	require.Equal(t, "^(?:)$", conf.IIS.AppExclude)
	require.Equal(t, "^(?:.+)$", conf.IIS.AppInclude)
	require.Equal(t, "C:", conf.TextFile.TextFileDirectory)
	require.Equal(t, "^(?:)$", conf.SMTP.Exclude)
	require.Equal(t, "^(?:.+)$", conf.SMTP.Include)
	require.Equal(t, "^(?:.*)$", conf.Service.Include)
	require.Equal(t, "^(?:alloy)$", conf.Service.Exclude)
	require.Equal(t, "^(?:)$", conf.PhysicalDisk.Exclude)
	require.Equal(t, "^(?:.+)$", conf.PhysicalDisk.Include)
	require.Equal(t, "^(?:)$", conf.Process.Exclude)
	require.Equal(t, "^(?:.+)$", conf.Process.Include)
	require.Equal(t, "^(?:)$", conf.Printer.Exclude)
	require.Equal(t, "^(?:.+)$", conf.Printer.Include)
	require.Equal(t, "example", conf.SMB.EnabledList)
	require.Equal(t, "example", conf.SMBClient.EnabledList)
	require.Equal(t, "^(?:)$", conf.Network.Exclude)
	require.Equal(t, "^(?:.+)$", conf.Network.Include)
	require.Equal(t, "accessmethods", conf.MSSQL.EnabledClasses)
	require.Equal(t, "^(?:)$", conf.LogicalDisk.Exclude)
	require.Equal(t, "^(?:.+)$", conf.LogicalDisk.Include)
	require.Equal(t, "example", conf.TCP.EnabledList)
	require.Equal(t, []string{"example"}, conf.Filetime.FilePatterns)
}
