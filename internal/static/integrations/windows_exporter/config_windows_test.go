package windows_exporter

import (
	"testing"

	"github.com/prometheus-community/windows_exporter/pkg/collector"
	"github.com/stretchr/testify/require"
)

func TestConfig_TextFileDefaultsWhenUnset(t *testing.T) {
	c := &Config{}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	require.Equal(t, collector.ConfigDefaults.Textfile.TextFileDirectories, cfg.Textfile.TextFileDirectories)
}

func TestConfig_TextFileExplicitDirectoryOverridesDefault(t *testing.T) {
	c := &Config{
		TextFile: TextFileConfig{TextFileDirectory: `C:\custom\path`},
	}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	require.Equal(t, []string{`C:\custom\path`}, cfg.Textfile.TextFileDirectories)
}

func TestConfig_FiletimeCollectorBecomesFile(t *testing.T) {
	c := &Config{
		EnabledCollectors: "cpu,filetime",
		Filetime:          FiletimeConfig{FilePatterns: []string{`C:\logs\*.log`}},
	}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	names, _ := normalizeCollectors(c.EnabledCollectors)
	require.Equal(t, []string{"cpu", "file"}, names)
	require.Equal(t, []string{`C:\logs\*.log`}, cfg.File.FilePatterns)
}

func TestConfig_FileAndFiletimePatternsMerge(t *testing.T) {
	c := &Config{
		EnabledCollectors: "file,filetime",
		File:              FileConfig{FilePatterns: []string{`C:\a\*`, `C:\b\*`}},
		Filetime:          FiletimeConfig{FilePatterns: []string{`C:\b\*`}},
	}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	names, _ := normalizeCollectors(c.EnabledCollectors)
	require.Equal(t, []string{"file"}, names)
	require.Equal(t, []string{`C:\a\*`, `C:\b\*`}, cfg.File.FilePatterns)
}

func TestConfig_ThermalzoneCollectorIsDropped(t *testing.T) {
	c := &Config{EnabledCollectors: "cpu,thermalzone"}

	_, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	names, _ := normalizeCollectors(c.EnabledCollectors)
	require.Equal(t, []string{"cpu"}, names)
}

func TestConfig_CRIEndpointDefaultsWhenUnset(t *testing.T) {
	c := &Config{}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	require.Equal(t, collector.ConfigDefaults.Container.CRIEndpoint, cfg.Container.CRIEndpoint)
}

func TestConfig_CRIEndpointAndMissedRuns(t *testing.T) {
	c := &Config{
		Container:     ContainerConfig{CRIEndpoint: "npipe:////./pipe/custom"},
		ScheduledTask: ScheduledTaskConfig{MissedRuns: true},
	}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	require.Equal(t, "npipe:////./pipe/custom", cfg.Container.CRIEndpoint)
	require.True(t, cfg.ScheduledTask.MissedRuns)
}

func TestConfig_RegistryAndWMI(t *testing.T) {
	c := &Config{
		Registry: RegistryConfig{Keys: `
- name: example
  key: 'HKLM\SOFTWARE\Example'
  values:
    - name: Version
      type: gauge
      metric: windows_example_version
`},
		WMI: WMIConfig{Queries: `
- name: example
  class: Win32_Process
  properties:
    - name: ProcessId
      type: gauge
      metric: windows_example_process_id
`},
	}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	require.Len(t, cfg.Registry.Keys, 1)
	require.Equal(t, "example", cfg.Registry.Keys[0].Name)
	require.Len(t, cfg.WMI.Queries, 1)
	require.Equal(t, "example", cfg.WMI.Queries[0].Name)
}

func TestConfig_RegistryAndWMIRejectUnknownKeys(t *testing.T) {
	_, err := (&Config{WMI: WMIConfig{Queries: "- name: example\n  query: SELECT 1\n"}}).ToWindowsExporterConfig()
	require.Error(t, err)

	_, err = (&Config{Registry: RegistryConfig{Keys: "- name: example\n  unknown: value\n"}}).ToWindowsExporterConfig()
	require.Error(t, err)
}

func TestConfig_EmptySubCollectorListsAreDropped(t *testing.T) {
	c := &Config{
		EnabledCollectors: "mscluster_node",
		TCP:               TCPConfig{EnabledList: ""},
		Dfsr:              DfsrConfig{SourcesEnabled: ""},
		MSCluster:         MSClusterConfig{EnabledList: ""},
	}

	cfg, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	require.Empty(t, cfg.TCP.CollectorsEnabled)
	require.Empty(t, cfg.DFSR.CollectorsEnabled)
	require.Equal(t, []string{"node"}, cfg.MSCluster.CollectorsEnabled)
}

func TestConfig_ToWindowsExporterConfigIsIdempotent(t *testing.T) {
	c := &Config{EnabledCollectors: "netframework_clrmemory,netframework_clrjit,cpu"}

	first, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	second, err := c.ToWindowsExporterConfig()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"clrmemory", "clrjit"}, first.NetFramework.CollectorsEnabled)
	require.ElementsMatch(t, first.NetFramework.CollectorsEnabled, second.NetFramework.CollectorsEnabled)
	require.Equal(t, "netframework_clrmemory,netframework_clrjit,cpu", c.EnabledCollectors)
}

func TestConfig_RegistryAndWMIAcceptEmptyDocuments(t *testing.T) {
	for _, doc := range []string{" \n  \n", "# - name: example\n", "---\n# nothing\n"} {
		cfg, err := (&Config{Registry: RegistryConfig{Keys: doc}, WMI: WMIConfig{Queries: doc}}).ToWindowsExporterConfig()
		require.NoError(t, err, "document %q", doc)
		require.Empty(t, cfg.Registry.Keys)
		require.Empty(t, cfg.WMI.Queries)
	}
}
