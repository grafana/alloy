package windows_exporter

import (
	"testing"
	"time"

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

func TestConfig_UpdateSettingsAreApplied(t *testing.T) {
	cfg, err := (&Config{Update: UpdateConfig{Online: true, ScrapeInterval: time.Hour}}).ToWindowsExporterConfig()
	require.NoError(t, err)
	require.True(t, cfg.Update.Online)
	require.Equal(t, time.Hour, cfg.Update.ScrapeInterval)

	cfg, err = (&Config{}).ToWindowsExporterConfig()
	require.NoError(t, err)
	require.Equal(t, collector.ConfigDefaults.Update.ScrapeInterval, cfg.Update.ScrapeInterval)
}
