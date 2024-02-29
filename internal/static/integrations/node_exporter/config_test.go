package node_exporter //nolint:golint

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestNodeExporter_Config(t *testing.T) {
	var c Config

	err := yaml.Unmarshal([]byte("{}"), &c)
	require.NoError(t, err)
	require.Equal(t, DefaultConfig, c)
}

func TestNodeExporter_ConfigMigrate(t *testing.T) {
	tt := []struct {
		name           string
		in             string
		expectError    string
		expectWarnings []string
		check          func(t *testing.T, c *Config)
	}{
		{
			name: "old fields migrate",
			in: `
      netdev_device_whitelist: netdev_wl
      netdev_device_blacklist: netdev_bl
      systemd_unit_whitelist: systemd_wl
      systemd_unit_blacklist: systemd_bl
      filesystem_ignored_mount_points: fs_mp
      filesystem_ignored_fs_types: fs_types
      diskstats_ignored_devices: diskstats_exclude
      `,
			expectWarnings: []string{
				`"netdev_device_whitelist" is deprecated by "netdev_device_include" and will be removed in a future version`,
				`"netdev_device_blacklist" is deprecated by "netdev_device_exclude" and will be removed in a future version`,
				`"systemd_unit_whitelist" is deprecated by "systemd_unit_include" and will be removed in a future version`,
				`"systemd_unit_blacklist" is deprecated by "systemd_unit_exclude" and will be removed in a future version`,
				`"filesystem_ignored_mount_points" is deprecated by "filesystem_mount_points_exclude" and will be removed in a future version`,
				`"filesystem_ignored_fs_types" is deprecated by "filesystem_fs_types_exclude" and will be removed in a future version`,
				`"diskstats_ignored_devices" is deprecated by "diskstats_device_exclude" and will be removed in a future version`,
			},
			check: func(t *testing.T, c *Config) {
				t.Helper()

				require.Equal(t, c.NetdevDeviceInclude, "netdev_wl")
				require.Equal(t, c.NetdevDeviceExclude, "netdev_bl")
				require.Equal(t, c.SystemdUnitInclude, "systemd_wl")
				require.Equal(t, c.SystemdUnitExclude, "systemd_bl")
				require.Equal(t, c.FilesystemMountPointsExclude, "fs_mp")
				require.Equal(t, c.FilesystemFSTypesExclude, "fs_types")
				require.Equal(t, c.DiskStatsDeviceExclude, "diskstats_exclude")
			},
		},
		{
			name: "new fields valid",
			in: `
      netdev_device_include: netdev_wl
      netdev_device_exclude: netdev_bl
      systemd_unit_include: systemd_wl
      systemd_unit_exclude: systemd_bl
      filesystem_mount_points_exclude: fs_mp
      filesystem_fs_types_exclude: fs_types
      diskstats_device_exclude: diskstats_exclude
      `,
			check: func(t *testing.T, c *Config) {
				t.Helper()

				require.Equal(t, c.NetdevDeviceInclude, "netdev_wl")
				require.Equal(t, c.NetdevDeviceExclude, "netdev_bl")
				require.Equal(t, c.SystemdUnitInclude, "systemd_wl")
				require.Equal(t, c.SystemdUnitExclude, "systemd_bl")
				require.Equal(t, c.FilesystemMountPointsExclude, "fs_mp")
				require.Equal(t, c.FilesystemFSTypesExclude, "fs_types")
				require.Equal(t, c.DiskStatsDeviceExclude, "diskstats_exclude")
			},
		},
		{
			name:        `netdev_device_whitelist and netdev_device_include`,
			in:          illegalConfig("netdev_device_whitelist", "netdev_device_include"),
			expectError: `only one of "netdev_device_whitelist" and "netdev_device_include" may be specified`,
		},
		{
			name:        `netdev_device_blacklist and netdev_device_exclude`,
			in:          illegalConfig("netdev_device_blacklist", "netdev_device_exclude"),
			expectError: `only one of "netdev_device_blacklist" and "netdev_device_exclude" may be specified`,
		},
		{
			name:        `systemd_unit_whitelist and systemd_unit_include`,
			in:          illegalConfig("systemd_unit_whitelist", "systemd_unit_include"),
			expectError: `only one of "systemd_unit_whitelist" and "systemd_unit_include" may be specified`,
		},
		{
			name:        `systemd_unit_blacklist and systemd_unit_exclude`,
			in:          illegalConfig("systemd_unit_blacklist", "systemd_unit_exclude"),
			expectError: `only one of "systemd_unit_blacklist" and "systemd_unit_exclude" may be specified`,
		},
		{
			name:        `filesystem_ignored_mount_points and filesystem_mount_points_exclude`,
			in:          illegalConfig("filesystem_ignored_mount_points", "filesystem_mount_points_exclude"),
			expectError: `only one of "filesystem_ignored_mount_points" and "filesystem_mount_points_exclude" may be specified`,
		},
		{
			name:        `filesystem_ignored_fs_types and filesystem_fs_types_exclude`,
			in:          illegalConfig("filesystem_ignored_fs_types", "filesystem_fs_types_exclude"),
			expectError: `only one of "filesystem_ignored_fs_types" and "filesystem_fs_types_exclude" may be specified`,
		},
		{
			name:        `diskstats_ignored_devices and diskstats_device_exclude`,
			in:          illegalConfig("diskstats_ignored_devices", "diskstats_device_exclude"),
			expectError: `only one of "diskstats_ignored_devices" and "diskstats_device_exclude" may be specified`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var c Config

			err := yaml.Unmarshal([]byte(tc.in), &c)
			if tc.expectError != "" {
				require.EqualError(t, err, tc.expectError)
				return
			}
			require.NoError(t, err)

			require.ElementsMatch(t, tc.expectWarnings, c.UnmarshalWarnings)
			if tc.check != nil {
				tc.check(t, &c)
			}
		})
	}
}

func illegalConfig(oldName, newName string) string {
	return fmt.Sprintf(`
  %s: illegal
  %s: illegal
  `, oldName, newName)
}
