package file

import (
	"testing"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/stretchr/testify/require"
)

func TestArguments_SetToDefault(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	require.Equal(t, "json", args.Format)
	require.Equal(t, time.Second, args.FlushInterval)
	require.Nil(t, args.Rotation)
	require.Nil(t, args.GroupBy)
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{
			name: "valid config",
			args: Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: time.Second},
		},
		{
			name:    "empty path",
			args:    Arguments{Path: "", Format: "json", FlushInterval: time.Second},
			wantErr: "path must be non-empty",
		},
		{
			name:    "append and compression",
			args:    Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: time.Second, Append: true, Compression: "zstd"},
			wantErr: "append and compression enabled at the same time is not supported",
		},
		{
			name:    "append and rotation",
			args:    Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: time.Second, Append: true, Rotation: &Rotation{MaxBackups: 5}},
			wantErr: "append and rotation enabled at the same time is not supported",
		},
		{
			name:    "invalid format",
			args:    Arguments{Path: "/tmp/test.json", Format: "xml", FlushInterval: time.Second},
			wantErr: "format type is not supported",
		},
		{
			name:    "invalid compression",
			args:    Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: time.Second, Compression: "gzip"},
			wantErr: "compression is not supported",
		},
		{
			name:    "negative flush interval",
			args:    Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: -time.Second},
			wantErr: "flush_interval must be larger than zero",
		},
		{
			name:    "zero flush interval",
			args:    Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: -1},
			wantErr: "flush_interval must be larger than zero",
		},
		{
			name:    "group_by without asterisk",
			args:    Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: time.Second, GroupBy: &GroupBy{Enabled: true, ResourceAttribute: "svc.name"}},
			wantErr: "path must contain exactly one * when group_by is enabled",
		},
		{
			name:    "group_by path starts with asterisk",
			args:    Arguments{Path: "*/tmp/test.json", Format: "json", FlushInterval: time.Second, GroupBy: &GroupBy{Enabled: true, ResourceAttribute: "svc.name"}},
			wantErr: "path must not start with * when group_by is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				if tt.name == "group_by defaults applied" {
					require.Equal(t, "fileexporter.path_segment", tt.args.GroupBy.ResourceAttribute)
					require.Equal(t, 100, tt.args.GroupBy.MaxOpenFiles)
				}
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestArguments_Convert_EncodingUnsupported(t *testing.T) {
	args := Arguments{Path: "/tmp/test.json", Format: "json", FlushInterval: time.Second, Encoding: "proto"}
	_, err := args.Convert()
	require.Error(t, err)
	require.Contains(t, err.Error(), "encoding parameter is not yet supported")
}

func TestArguments_Convert(t *testing.T) {
	args := Arguments{
		Path:          "/tmp/*/test.json",
		Format:        "json",
		Append:        false,
		Compression:   "zstd",
		FlushInterval: 5 * time.Second,
		Rotation: &Rotation{
			MaxMegabytes: 50,
			MaxDays:      7,
			MaxBackups:   10,
			LocalTime:    true,
		},
		GroupBy: &GroupBy{
			Enabled:           true,
			ResourceAttribute: "service.name",
			MaxOpenFiles:      50,
		},
	}

	cfg, err := args.Convert()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.NoError(t, args.Validate())

	fileCfg := cfg.(*fileexporter.Config)
	require.Equal(t, "/tmp/*/test.json", fileCfg.Path)
	require.Equal(t, "json", fileCfg.FormatType)
	require.False(t, fileCfg.Append)
	require.Equal(t, "zstd", fileCfg.Compression)
	require.Equal(t, 5*time.Second, fileCfg.FlushInterval)

	require.NotNil(t, fileCfg.Rotation)
	require.Equal(t, 50, fileCfg.Rotation.MaxMegabytes)
	require.Equal(t, 7, fileCfg.Rotation.MaxDays)
	require.Equal(t, 10, fileCfg.Rotation.MaxBackups)
	require.True(t, fileCfg.Rotation.LocalTime)

	require.NotNil(t, fileCfg.GroupBy)
	require.True(t, fileCfg.GroupBy.Enabled)
	require.Equal(t, "service.name", fileCfg.GroupBy.ResourceAttribute)
	require.Equal(t, 50, fileCfg.GroupBy.MaxOpenFiles)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	alloyCfg := `
path = "/tmp/*/test.json"
format = "json"
append = false
compression = "zstd"
flush_interval = "5s"

rotation {
  max_megabytes = 50
  max_days = 7
  max_backups = 10
  localtime = true
}

group_by {
  enabled = true
  resource_attribute = "service.name"
  max_open_files = 50
}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	require.NoError(t, args.Validate())

	require.Equal(t, "/tmp/*/test.json", args.Path)
	require.Equal(t, "json", args.Format)
	require.False(t, args.Append)
	require.Equal(t, "zstd", args.Compression)
	require.Equal(t, 5*time.Second, args.FlushInterval)

	require.NotNil(t, args.Rotation)
	require.Equal(t, 50, args.Rotation.MaxMegabytes)
	require.Equal(t, 7, args.Rotation.MaxDays)
	require.Equal(t, 10, args.Rotation.MaxBackups)
	require.True(t, args.Rotation.LocalTime)

	require.NotNil(t, args.GroupBy)
	require.True(t, args.GroupBy.Enabled)
	require.Equal(t, "service.name", args.GroupBy.ResourceAttribute)
	require.Equal(t, 50, args.GroupBy.MaxOpenFiles)
}

func TestArguments_UnmarshalAlloyWithGroupByDefaults(t *testing.T) {
	alloyCfg := `
path = "/tmp/*/test.json"
format = "json"
append = false
compression = "zstd"
flush_interval = "5s"

rotation {
  max_megabytes = 50
  max_days = 7
  max_backups = 10
  localtime = true
}

group_by {
  enabled = true
}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	require.NoError(t, args.Validate())

	require.Equal(t, "/tmp/*/test.json", args.Path)
	require.Equal(t, "json", args.Format)
	require.False(t, args.Append)
	require.Equal(t, "zstd", args.Compression)
	require.Equal(t, 5*time.Second, args.FlushInterval)

	require.NotNil(t, args.Rotation)
	require.Equal(t, 50, args.Rotation.MaxMegabytes)
	require.Equal(t, 7, args.Rotation.MaxDays)
	require.Equal(t, 10, args.Rotation.MaxBackups)
	require.True(t, args.Rotation.LocalTime)

	require.NotNil(t, args.GroupBy)
	require.True(t, args.GroupBy.Enabled)
	require.Equal(t, "fileexporter.path_segment", args.GroupBy.ResourceAttribute)
	require.Equal(t, 100, args.GroupBy.MaxOpenFiles)
}
