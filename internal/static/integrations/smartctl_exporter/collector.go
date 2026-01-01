package smartctl_exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

// smartctlCollector implements prometheus.Collector for S.M.A.R.T. metrics
type smartctlCollector struct {
	cfg    *Config
	logger log.Logger

	// Version metrics
	smartctlVersion *prometheus.Desc

	// Device info and status
	deviceInfo          *prometheus.Desc
	deviceSmartStatus   *prometheus.Desc
	deviceStatus        *prometheus.Desc
	deviceExitStatus    *prometheus.Desc

	// Capacity metrics
	deviceCapacityBlocks *prometheus.Desc
	deviceCapacityBytes  *prometheus.Desc
	deviceBlockSize      *prometheus.Desc
	deviceRotationRate   *prometheus.Desc

	// Temperature
	deviceTemperature *prometheus.Desc

	// Power metrics
	devicePowerOnSeconds *prometheus.Desc
	devicePowerCycleCount *prometheus.Desc

	// Data transfer metrics
	deviceBytesRead    *prometheus.Desc
	deviceBytesWritten *prometheus.Desc

	// Error metrics
	deviceMediaErrors      *prometheus.Desc
	deviceNumErrLogEntries *prometheus.Desc

	// NVMe-specific metrics
	devicePercentageUsed         *prometheus.Desc
	deviceAvailableSpare         *prometheus.Desc
	deviceAvailableSpareThreshold *prometheus.Desc
	deviceCriticalWarning        *prometheus.Desc

	// Interface speed
	deviceInterfaceSpeed *prometheus.Desc

	// SMART attributes
	deviceAttribute *prometheus.Desc

	// Scrape metrics
	deviceScrapeSuccess  *prometheus.Desc
	deviceScrapeDuration *prometheus.Desc

	// Device management
	mu      sync.RWMutex
	devices []string

	// Filtering
	includeFilter *regexp.Regexp
	excludeFilter *regexp.Regexp
}

// attrFlags represents SMART attribute flags
type attrFlags struct {
	Value         int    `json:"value"`
	String        string `json:"string"`
	Prefailure    bool   `json:"prefailure"`
	UpdatedOnline bool   `json:"updated_online"`
	Performance   bool   `json:"performance"`
	ErrorRate     bool   `json:"error_rate"`
	EventCount    bool   `json:"event_count"`
	AutoKeep      bool   `json:"auto_keep"`
}

// smartctlDevice represents the full JSON output from smartctl
type smartctlDevice struct {
	Device struct {
		Name     string `json:"name"`
		InfoName string `json:"info_name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelName        string `json:"model_name"`
	ModelFamily      string `json:"model_family"`
	SerialNumber     string `json:"serial_number"`
	FirmwareVersion  string `json:"firmware_version"`
	UserCapacity     *struct {
		Blocks int64 `json:"blocks"`
		Bytes  int64 `json:"bytes"`
	} `json:"user_capacity"`
	LogicalBlockSize  int `json:"logical_block_size"`
	PhysicalBlockSize int `json:"physical_block_size"`
	FormFactor        *struct {
		Name string `json:"name"`
	} `json:"form_factor"`
	RotationRate    int    `json:"rotation_rate"`
	ATAVersion      *struct {
		String string `json:"string"`
	} `json:"ata_version"`
	SATAVersion     *struct {
		String string `json:"string"`
	} `json:"sata_version"`
	InterfaceSpeed  *struct {
		Max struct {
			UnitsPerSecond int64  `json:"units_per_second"`
			String         string `json:"string"`
		} `json:"max"`
		Current struct {
			UnitsPerSecond int64  `json:"units_per_second"`
			String         string `json:"string"`
		} `json:"current"`
	} `json:"interface_speed"`
	SmartStatus struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	SmartctlExitStatus int `json:"smartctl_exit_status"`
	Temperature        *struct {
		Current int `json:"current"`
	} `json:"temperature"`
	PowerOnTime *struct {
		Hours int64 `json:"hours"`
	} `json:"power_on_time"`
	PowerCycleCount int64 `json:"power_cycle_count"`

	// SMART attributes (ATA/SATA)
	AtaSmartAttributes *struct {
		Table []struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Value      int    `json:"value"`
			Worst      int    `json:"worst"`
			Thresh     int    `json:"thresh"`
			WhenFailed string `json:"when_failed"`
			Flags      attrFlags `json:"flags"`
			Raw struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`

	// NVMe-specific fields
	NvmeSmartHealthInformationLog *struct {
		Temperature                    int   `json:"temperature"`
		AvailableSpare                 int   `json:"available_spare"`
		AvailableSpareThreshold        int   `json:"available_spare_threshold"`
		PercentageUsed                 int   `json:"percentage_used"`
		CriticalWarning                int   `json:"critical_warning"`
		MediaErrors                    int64 `json:"media_errors"`
		NumErrLogEntries               int64 `json:"num_err_log_entries"`
		PowerOnHours                   int64 `json:"power_on_hours"`
		PowerCycles                    int64 `json:"power_cycles"`
		DataUnitsRead                  int64 `json:"data_units_read"`
		DataUnitsWritten               int64 `json:"data_units_written"`
	} `json:"nvme_smart_health_information_log"`
}

// newSmartctlCollector creates a new smartctl collector
func newSmartctlCollector(logger log.Logger, cfg *Config) (*smartctlCollector, error) {
	c := &smartctlCollector{
		cfg:    cfg,
		logger: logger,

		smartctlVersion: prometheus.NewDesc(
			"smartctl_version",
			"Smartctl version information",
			[]string{"version", "json_format_version"},
			nil,
		),

		deviceInfo: prometheus.NewDesc(
			"smartctl_device",
			"Device information",
			[]string{"device", "model_name", "model_family", "serial_number", "firmware_version", "interface", "protocol", "form_factor", "ata_version", "sata_version"},
			nil,
		),

		deviceSmartStatus: prometheus.NewDesc(
			"smartctl_device_smart_status",
			"Device SMART overall-health self-assessment test result (1 = PASSED, 0 = FAILED)",
			[]string{"device"},
			nil,
		),

		deviceStatus: prometheus.NewDesc(
			"smartctl_device_status",
			"Device status (1 = available, 0 = unavailable)",
			[]string{"device"},
			nil,
		),

		deviceExitStatus: prometheus.NewDesc(
			"smartctl_device_smartctl_exit_status",
			"Exit status from smartctl",
			[]string{"device"},
			nil,
		),

		deviceCapacityBlocks: prometheus.NewDesc(
			"smartctl_device_capacity_blocks",
			"Device capacity in blocks",
			[]string{"device"},
			nil,
		),

		deviceCapacityBytes: prometheus.NewDesc(
			"smartctl_device_capacity_bytes",
			"Device capacity in bytes",
			[]string{"device"},
			nil,
		),

		deviceBlockSize: prometheus.NewDesc(
			"smartctl_device_block_size",
			"Device block size in bytes",
			[]string{"device", "blocks_type"},
			nil,
		),

		deviceRotationRate: prometheus.NewDesc(
			"smartctl_device_rotation_rate",
			"Device rotation rate in RPM (0 for SSD)",
			[]string{"device"},
			nil,
		),

		deviceTemperature: prometheus.NewDesc(
			"smartctl_device_temperature",
			"Device temperature in Celsius",
			[]string{"device", "temperature_type"},
			nil,
		),

		devicePowerOnSeconds: prometheus.NewDesc(
			"smartctl_device_power_on_seconds",
			"Device power-on time in seconds",
			[]string{"device"},
			nil,
		),

		devicePowerCycleCount: prometheus.NewDesc(
			"smartctl_device_power_cycle_count",
			"Device power cycle count",
			[]string{"device"},
			nil,
		),

		deviceBytesRead: prometheus.NewDesc(
			"smartctl_device_bytes_read",
			"Total bytes read from device",
			[]string{"device"},
			nil,
		),

		deviceBytesWritten: prometheus.NewDesc(
			"smartctl_device_bytes_written",
			"Total bytes written to device",
			[]string{"device"},
			nil,
		),

		deviceMediaErrors: prometheus.NewDesc(
			"smartctl_device_media_errors",
			"Device media errors",
			[]string{"device"},
			nil,
		),

		deviceNumErrLogEntries: prometheus.NewDesc(
			"smartctl_device_num_err_log_entries",
			"Number of error log entries",
			[]string{"device"},
			nil,
		),

		devicePercentageUsed: prometheus.NewDesc(
			"smartctl_device_percentage_used",
			"Percentage of device lifespan used (NVMe)",
			[]string{"device"},
			nil,
		),

		deviceAvailableSpare: prometheus.NewDesc(
			"smartctl_device_available_spare",
			"Available spare capacity percentage (NVMe)",
			[]string{"device"},
			nil,
		),

		deviceAvailableSpareThreshold: prometheus.NewDesc(
			"smartctl_device_available_spare_threshold",
			"Available spare threshold percentage (NVMe)",
			[]string{"device"},
			nil,
		),

		deviceCriticalWarning: prometheus.NewDesc(
			"smartctl_device_critical_warning",
			"Critical warning status (NVMe)",
			[]string{"device"},
			nil,
		),

		deviceInterfaceSpeed: prometheus.NewDesc(
			"smartctl_device_interface_speed",
			"Device interface speed in units per second",
			[]string{"device", "speed_type"},
			nil,
		),

		deviceAttribute: prometheus.NewDesc(
			"smartctl_device_attribute",
			"SMART attribute values",
			[]string{"device", "attribute_id", "attribute_name", "attribute_value_type", "attribute_flags_short", "attribute_flags_long"},
			nil,
		),

		deviceScrapeSuccess: prometheus.NewDesc(
			"smartctl_device_scrape_success",
			"Whether the smartctl scrape was successful (1 = success, 0 = failure)",
			[]string{"device"},
			nil,
		),

		deviceScrapeDuration: prometheus.NewDesc(
			"smartctl_device_scrape_duration_seconds",
			"Duration of the smartctl scrape in seconds",
			[]string{"device"},
			nil,
		),
	}

	// Compile regex filters if provided
	var err error
	if cfg.DeviceInclude != "" {
		c.includeFilter, err = regexp.Compile(cfg.DeviceInclude)
		if err != nil {
			return nil, fmt.Errorf("invalid device_include regex: %w", err)
		}
	}
	if cfg.DeviceExclude != "" {
		c.excludeFilter, err = regexp.Compile(cfg.DeviceExclude)
		if err != nil {
			return nil, fmt.Errorf("invalid device_exclude regex: %w", err)
		}
	}

	// Initialize device list
	if len(cfg.Devices) > 0 {
		c.devices = cfg.Devices
	} else {
		// Discover devices
		devices, err := c.discoverDevices()
		if err != nil {
			level.Warn(logger).Log("msg", "initial device discovery failed, will retry", "err", err)
		} else {
			c.devices = devices
		}
	}

	return c, nil
}

// Describe implements prometheus.Collector
func (c *smartctlCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.smartctlVersion
	ch <- c.deviceInfo
	ch <- c.deviceSmartStatus
	ch <- c.deviceStatus
	ch <- c.deviceExitStatus
	ch <- c.deviceCapacityBlocks
	ch <- c.deviceCapacityBytes
	ch <- c.deviceBlockSize
	ch <- c.deviceRotationRate
	ch <- c.deviceTemperature
	ch <- c.devicePowerOnSeconds
	ch <- c.devicePowerCycleCount
	ch <- c.deviceBytesRead
	ch <- c.deviceBytesWritten
	ch <- c.deviceMediaErrors
	ch <- c.deviceNumErrLogEntries
	ch <- c.devicePercentageUsed
	ch <- c.deviceAvailableSpare
	ch <- c.deviceAvailableSpareThreshold
	ch <- c.deviceCriticalWarning
	ch <- c.deviceInterfaceSpeed
	ch <- c.deviceAttribute
	ch <- c.deviceScrapeSuccess
	ch <- c.deviceScrapeDuration
}

// Collect implements prometheus.Collector
func (c *smartctlCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	devices := make([]string, len(c.devices))
	copy(devices, c.devices)
	c.mu.RUnlock()

	// Emit version metric once
	ch <- prometheus.MustNewConstMetric(
		c.smartctlVersion,
		prometheus.GaugeValue,
		1,
		"7.0+", // Minimum required version
		"2",     // JSON format version
	)

	for _, device := range devices {
		c.collectDevice(ch, device)
	}
}

// collectDevice collects metrics for a single device
func (c *smartctlCollector) collectDevice(ch chan<- prometheus.Metric, device string) {
	startTime := time.Now()

	// Get smartctl data
	data, err := c.getSmartctlData(device)
	duration := time.Since(startTime).Seconds()

	ch <- prometheus.MustNewConstMetric(
		c.deviceScrapeDuration,
		prometheus.GaugeValue,
		duration,
		device,
	)

	if err != nil {
		level.Debug(c.logger).Log("msg", "failed to get smartctl data", "device", device, "err", err)
		ch <- prometheus.MustNewConstMetric(
			c.deviceScrapeSuccess,
			prometheus.GaugeValue,
			0,
			device,
		)
		ch <- prometheus.MustNewConstMetric(
			c.deviceStatus,
			prometheus.GaugeValue,
			0,
			device,
		)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		c.deviceScrapeSuccess,
		prometheus.GaugeValue,
		1,
		device,
	)

	ch <- prometheus.MustNewConstMetric(
		c.deviceStatus,
		prometheus.GaugeValue,
		1,
		device,
	)

	// Device info
	modelFamily := data.ModelFamily
	formFactor := ""
	if data.FormFactor != nil {
		formFactor = data.FormFactor.Name
	}
	ataVersion := ""
	if data.ATAVersion != nil {
		ataVersion = data.ATAVersion.String
	}
	sataVersion := ""
	if data.SATAVersion != nil {
		sataVersion = data.SATAVersion.String
	}

	ch <- prometheus.MustNewConstMetric(
		c.deviceInfo,
		prometheus.GaugeValue,
		1,
		device,
		data.ModelName,
		modelFamily,
		data.SerialNumber,
		data.FirmwareVersion,
		data.Device.Type,
		data.Device.Protocol,
		formFactor,
		ataVersion,
		sataVersion,
	)

	// SMART status
	smartStatusValue := float64(0)
	if data.SmartStatus.Passed {
		smartStatusValue = 1
	}
	ch <- prometheus.MustNewConstMetric(
		c.deviceSmartStatus,
		prometheus.GaugeValue,
		smartStatusValue,
		device,
	)

	// Exit status
	ch <- prometheus.MustNewConstMetric(
		c.deviceExitStatus,
		prometheus.GaugeValue,
		float64(data.SmartctlExitStatus),
		device,
	)

	// Capacity
	if data.UserCapacity != nil {
		ch <- prometheus.MustNewConstMetric(
			c.deviceCapacityBlocks,
			prometheus.GaugeValue,
			float64(data.UserCapacity.Blocks),
			device,
		)
		ch <- prometheus.MustNewConstMetric(
			c.deviceCapacityBytes,
			prometheus.GaugeValue,
			float64(data.UserCapacity.Bytes),
			device,
		)
	}

	// Block sizes
	if data.LogicalBlockSize > 0 {
		ch <- prometheus.MustNewConstMetric(
			c.deviceBlockSize,
			prometheus.GaugeValue,
			float64(data.LogicalBlockSize),
			device,
			"logical",
		)
	}
	if data.PhysicalBlockSize > 0 {
		ch <- prometheus.MustNewConstMetric(
			c.deviceBlockSize,
			prometheus.GaugeValue,
			float64(data.PhysicalBlockSize),
			device,
			"physical",
		)
	}

	// Rotation rate
	ch <- prometheus.MustNewConstMetric(
		c.deviceRotationRate,
		prometheus.GaugeValue,
		float64(data.RotationRate),
		device,
	)

	// Interface speed
	if data.InterfaceSpeed != nil {
		ch <- prometheus.MustNewConstMetric(
			c.deviceInterfaceSpeed,
			prometheus.GaugeValue,
			float64(data.InterfaceSpeed.Max.UnitsPerSecond),
			device,
			"max",
		)
		ch <- prometheus.MustNewConstMetric(
			c.deviceInterfaceSpeed,
			prometheus.GaugeValue,
			float64(data.InterfaceSpeed.Current.UnitsPerSecond),
			device,
			"current",
		)
	}

	// Temperature
	// NVMe devices have temperature in NvmeSmartHealthInformationLog
	// ATA/SATA devices have it in the general Temperature field
	if data.NvmeSmartHealthInformationLog != nil {
		ch <- prometheus.MustNewConstMetric(
			c.deviceTemperature,
			prometheus.GaugeValue,
			float64(data.NvmeSmartHealthInformationLog.Temperature),
			device,
			"current",
		)
	} else if data.Temperature != nil && data.Temperature.Current > 0 {
		ch <- prometheus.MustNewConstMetric(
			c.deviceTemperature,
			prometheus.GaugeValue,
			float64(data.Temperature.Current),
			device,
			"current",
		)
	}

	// NVMe-specific metrics
	if data.NvmeSmartHealthInformationLog != nil {
		nvme := data.NvmeSmartHealthInformationLog

		ch <- prometheus.MustNewConstMetric(
			c.deviceAvailableSpare,
			prometheus.CounterValue,
			float64(nvme.AvailableSpare),
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.deviceAvailableSpareThreshold,
			prometheus.CounterValue,
			float64(nvme.AvailableSpareThreshold),
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.devicePercentageUsed,
			prometheus.CounterValue,
			float64(nvme.PercentageUsed),
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.deviceCriticalWarning,
			prometheus.CounterValue,
			float64(nvme.CriticalWarning),
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.deviceMediaErrors,
			prometheus.CounterValue,
			float64(nvme.MediaErrors),
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.deviceNumErrLogEntries,
			prometheus.CounterValue,
			float64(nvme.NumErrLogEntries),
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.devicePowerOnSeconds,
			prometheus.CounterValue,
			float64(nvme.PowerOnHours*3600), // Convert hours to seconds
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.devicePowerCycleCount,
			prometheus.CounterValue,
			float64(nvme.PowerCycles),
			device,
		)

		// NVMe data units are in 512-byte blocks (per spec)
		ch <- prometheus.MustNewConstMetric(
			c.deviceBytesRead,
			prometheus.CounterValue,
			float64(nvme.DataUnitsRead*512),
			device,
		)

		ch <- prometheus.MustNewConstMetric(
			c.deviceBytesWritten,
			prometheus.CounterValue,
			float64(nvme.DataUnitsWritten*512),
			device,
		)
	} else {
		// ATA/SATA metrics
		if data.PowerOnTime != nil && data.PowerOnTime.Hours > 0 {
			ch <- prometheus.MustNewConstMetric(
				c.devicePowerOnSeconds,
				prometheus.CounterValue,
				float64(data.PowerOnTime.Hours*3600), // Convert hours to seconds
				device,
			)
		}

		if data.PowerCycleCount > 0 {
			ch <- prometheus.MustNewConstMetric(
				c.devicePowerCycleCount,
				prometheus.CounterValue,
				float64(data.PowerCycleCount),
				device,
			)
		}
	}

	// SMART attributes (ATA/SATA)
	if data.AtaSmartAttributes != nil {
		for _, attr := range data.AtaSmartAttributes.Table {
			// Emit normalized value
			ch <- prometheus.MustNewConstMetric(
				c.deviceAttribute,
				prometheus.GaugeValue,
				float64(attr.Value),
				device,
				strconv.Itoa(attr.ID),
				attr.Name,
				"normalized",
				attr.Flags.String,
				buildFlagsLong(attr.Flags),
			)

			// Emit raw value
			ch <- prometheus.MustNewConstMetric(
				c.deviceAttribute,
				prometheus.GaugeValue,
				float64(attr.Raw.Value),
				device,
				strconv.Itoa(attr.ID),
				attr.Name,
				"raw",
				attr.Flags.String,
				buildFlagsLong(attr.Flags),
			)

			// Emit worst value
			ch <- prometheus.MustNewConstMetric(
				c.deviceAttribute,
				prometheus.GaugeValue,
				float64(attr.Worst),
				device,
				strconv.Itoa(attr.ID),
				attr.Name,
				"worst",
				attr.Flags.String,
				buildFlagsLong(attr.Flags),
			)

			// Emit threshold value
			ch <- prometheus.MustNewConstMetric(
				c.deviceAttribute,
				prometheus.GaugeValue,
				float64(attr.Thresh),
				device,
				strconv.Itoa(attr.ID),
				attr.Name,
				"threshold",
				attr.Flags.String,
				buildFlagsLong(attr.Flags),
			)
		}
	}
}

// buildFlagsLong builds a long-form flags string
func buildFlagsLong(flags attrFlags) string {
	var parts []string
	if flags.Prefailure {
		parts = append(parts, "prefailure")
	}
	if flags.UpdatedOnline {
		parts = append(parts, "updated_online")
	}
	if flags.Performance {
		parts = append(parts, "performance")
	}
	if flags.ErrorRate {
		parts = append(parts, "error_rate")
	}
	if flags.EventCount {
		parts = append(parts, "event_count")
	}
	if flags.AutoKeep {
		parts = append(parts, "auto_keep")
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ","
		}
		result += p
	}
	return result
}

// getSmartctlData executes smartctl and parses its JSON output
func (c *smartctlCollector) getSmartctlData(device string) (*smartctlDevice, error) {
	// Build smartctl command
	// -j = JSON output, -a = all information
	args := []string{"-j", "-a"}

	// Add device type if specified
	if len(c.cfg.ScanDeviceTypes) > 0 && c.cfg.ScanDeviceTypes[0] != "auto" {
		args = append(args, "-d", c.cfg.ScanDeviceTypes[0])
	}

	// Check power mode if configured
	if c.cfg.PowermodeCheck != "never" {
		args = append(args, "-n", c.cfg.PowermodeCheck)
	}

	args = append(args, device)

	// Execute smartctl
	cmd := exec.Command(c.cfg.SmartctlPath, args...)
	output, err := cmd.Output()

	// Note: smartctl returns non-zero exit codes for various conditions
	// We should still try to parse the JSON output
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("smartctl execution failed: %w", err)
	}

	// Parse JSON output
	var data smartctlDevice
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse smartctl JSON: %w", err)
	}

	return &data, nil
}

// discoverDevices finds available storage devices
func (c *smartctlCollector) discoverDevices() ([]string, error) {
	// Execute smartctl --scan-open to discover devices
	cmd := exec.Command(c.cfg.SmartctlPath, "--scan-open", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("device scan failed: %w", err)
	}

	// Parse scan output
	var scanResult struct {
		Devices []struct {
			Name string `json:"name"`
		} `json:"devices"`
	}

	if err := json.Unmarshal(output, &scanResult); err != nil {
		return nil, fmt.Errorf("failed to parse scan JSON: %w", err)
	}

	var devices []string
	for _, dev := range scanResult.Devices {
		// Apply filters
		if c.shouldIncludeDevice(dev.Name) {
			devices = append(devices, dev.Name)
		}
	}

	level.Debug(c.logger).Log("msg", "discovered devices", "count", len(devices), "devices", fmt.Sprintf("%v", devices))

	return devices, nil
}

// shouldIncludeDevice checks if a device should be included based on filters
func (c *smartctlCollector) shouldIncludeDevice(device string) bool {
	if c.includeFilter != nil {
		return c.includeFilter.MatchString(device)
	}
	if c.excludeFilter != nil {
		return !c.excludeFilter.MatchString(device)
	}
	return true
}

// Run starts the background scanning process
func (c *smartctlCollector) Run(ctx context.Context) error {
	// If no automatic rescanning, just wait for cancellation
	if c.cfg.RescanInterval == 0 || len(c.cfg.Devices) > 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	// Start periodic device rescanning
	ticker := time.NewTicker(c.cfg.RescanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			devices, err := c.discoverDevices()
			if err != nil {
				level.Warn(c.logger).Log("msg", "device rescan failed", "err", err)
				continue
			}

			c.mu.Lock()
			c.devices = devices
			c.mu.Unlock()

			level.Debug(c.logger).Log("msg", "device list updated", "count", len(devices))
		}
	}
}
