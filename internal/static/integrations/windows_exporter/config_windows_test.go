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
