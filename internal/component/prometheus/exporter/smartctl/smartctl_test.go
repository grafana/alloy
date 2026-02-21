package smartctl

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/static/integrations/smartctl_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfigUnmarshal(t *testing.T) {
	var exampleAlloyConfig = `
	smartctl_path = "/usr/local/bin/smartctl"
	scan_interval = "30s"
	rescan_interval = "5m"
	devices = ["/dev/sda", "/dev/nvme0n1"]
	device_exclude = "^(ram|loop)\\d+$"
	scan_device_types = ["sat", "nvme"]
	powermode_check = "idle"
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	require.Equal(t, "/usr/local/bin/smartctl", args.SmartctlPath)
	require.Equal(t, 30*time.Second, args.ScanInterval)
	require.Equal(t, 5*time.Minute, args.RescanInterval)
	require.Equal(t, []string{"/dev/sda", "/dev/nvme0n1"}, args.Devices)
	require.Equal(t, "^(ram|loop)\\d+$", args.DeviceExclude)
	require.Equal(t, []string{"sat", "nvme"}, args.ScanDeviceTypes)
	require.Equal(t, "idle", args.PowermodeCheck)
}

func TestAlloyConfigUnmarshalDefaults(t *testing.T) {
	var emptyConfig = ``

	var args Arguments
	err := syntax.Unmarshal([]byte(emptyConfig), &args)
	require.NoError(t, err)

	// Should have defaults after unmarshaling empty config
	args.SetToDefault()
	require.Equal(t, "/usr/sbin/smartctl", args.SmartctlPath)
	require.Equal(t, 60*time.Second, args.ScanInterval)
	require.Equal(t, 10*time.Minute, args.RescanInterval)
	require.Equal(t, "standby", args.PowermodeCheck)
}

func TestAlloyConfigConvert(t *testing.T) {
	var exampleAlloyConfig = `
	smartctl_path = "/usr/local/bin/smartctl"
	scan_interval = "30s"
	rescan_interval = "5m"
	devices = ["/dev/sda", "/dev/nvme0n1"]
	device_exclude = "^(ram|loop)\\d+$"
	scan_device_types = ["sat", "nvme"]
	powermode_check = "idle"
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	c := args.Convert()
	require.Equal(t, "/usr/local/bin/smartctl", c.SmartctlPath)
	require.Equal(t, 30*time.Second, c.ScanInterval)
	require.Equal(t, 5*time.Minute, c.RescanInterval)
	require.Equal(t, []string{"/dev/sda", "/dev/nvme0n1"}, c.Devices)
	require.Equal(t, "^(ram|loop)\\d+$", c.DeviceExclude)
	require.Equal(t, []string{"sat", "nvme"}, c.ScanDeviceTypes)
	require.Equal(t, "idle", c.PowermodeCheck)
}

// Checks that the configs have not drifted between Grafana Agent static mode and Alloy.
func TestDefaultsSame(t *testing.T) {
	convertedDefaults := DefaultArguments.Convert()
	require.Equal(t, smartctl_exporter.DefaultConfig.SmartctlPath, convertedDefaults.SmartctlPath)
	require.Equal(t, smartctl_exporter.DefaultConfig.ScanInterval, convertedDefaults.ScanInterval)
	require.Equal(t, smartctl_exporter.DefaultConfig.RescanInterval, convertedDefaults.RescanInterval)
	require.Equal(t, smartctl_exporter.DefaultConfig.PowermodeCheck, convertedDefaults.PowermodeCheck)
}

func TestValidate_MutuallyExclusiveFilters(t *testing.T) {
	args := Arguments{
		DeviceInclude: "^sd",
		DeviceExclude: "^loop",
	}
	require.Error(t, args.Validate())
	require.Contains(t, args.Validate().Error(), "mutually exclusive")
}

func TestValidate_InvalidPowermode(t *testing.T) {
	args := Arguments{
		PowermodeCheck: "invalid",
	}
	require.Error(t, args.Validate())
	require.Contains(t, args.Validate().Error(), "invalid powermode_check")
}

func TestValidate_ValidPowermodes(t *testing.T) {
	validModes := []string{"never", "sleep", "standby", "idle"}

	for _, mode := range validModes {
		t.Run("powermode="+mode, func(t *testing.T) {
			args := Arguments{
				PowermodeCheck: mode,
			}
			require.NoError(t, args.Validate())
		})
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	args := Arguments{
		SmartctlPath:    "/usr/sbin/smartctl",
		ScanInterval:    60 * time.Second,
		RescanInterval:  10 * time.Minute,
		DeviceExclude:   "^(ram|loop)\\d+$",
		PowermodeCheck:  "standby",
		ScanDeviceTypes: []string{"sat"},
	}
	require.NoError(t, args.Validate())
}

func TestValidate_DeviceIncludeOnly(t *testing.T) {
	args := Arguments{
		DeviceInclude: "^sd",
	}
	require.NoError(t, args.Validate())
}

func TestValidate_DeviceExcludeOnly(t *testing.T) {
	args := Arguments{
		DeviceExclude: "^loop",
	}
	require.NoError(t, args.Validate())
}

func TestSetToDefault(t *testing.T) {
	args := Arguments{
		SmartctlPath:   "/custom/path",
		ScanInterval:   30 * time.Second,
		RescanInterval: 5 * time.Minute,
	}

	args.SetToDefault()

	require.Equal(t, DefaultArguments.SmartctlPath, args.SmartctlPath)
	require.Equal(t, DefaultArguments.ScanInterval, args.ScanInterval)
	require.Equal(t, DefaultArguments.RescanInterval, args.RescanInterval)
	require.Equal(t, DefaultArguments.PowermodeCheck, args.PowermodeCheck)
}
