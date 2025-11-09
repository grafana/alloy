// The IPMI exporter integration collects hardware metrics from IPMI-enabled devices.
// It supports both local IPMI collection (from the machine running Alloy) and remote
// IPMI collection from network-accessible devices.
package ipmi_exporter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bougou/go-ipmi"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/integrations"
	config_integrations "github.com/grafana/alloy/internal/static/integrations/config"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/grafana/alloy/internal/util"
)

// DefaultConfig holds the default settings for the ipmi_exporter integration.
var DefaultConfig = Config{
	Timeout: 30000, // 30 seconds in milliseconds (IPMI sensor collection can be slow)
}

// IPMITarget defines a target device to be monitored.
type IPMITarget struct {
	Name   string `yaml:"name"`
	Target string `yaml:"target"`
	Module string `yaml:"module,omitempty"`

	// Authentication for remote IPMI (optional, can be defined in module config instead)
	User      string             `yaml:"user,omitempty"`
	Password  config_util.Secret `yaml:"password,omitempty"`
	Driver    string             `yaml:"driver,omitempty"`    // LAN_2_0 or LAN
	Privilege string             `yaml:"privilege,omitempty"` // user or admin
}

// LocalConfig controls local IPMI collection.
type LocalConfig struct {
	// Enabled controls whether local IPMI collection is enabled.
	Enabled bool `yaml:"enabled,omitempty"`

	// Module specifies which collector module to use for local collection.
	Module string `yaml:"module,omitempty"`
}

// Config controls the ipmi_exporter integration.
type Config struct {
	// Local configures monitoring of the local machine's IPMI interface.
	Local LocalConfig `yaml:"local,omitempty"`

	// Targets to monitor via remote IPMI.
	Targets []IPMITarget `yaml:"targets,omitempty"`

	// Timeout for IPMI requests in milliseconds.
	Timeout int64 `yaml:"timeout,omitempty"`

	// ConfigFile points to an external ipmi_exporter configuration file.
	// This file configures collectors, modules, and command overrides.
	// See https://github.com/prometheus-community/ipmi_exporter for examples.
	ConfigFile string `yaml:"config_file,omitempty"`

	// IPMIConfig is the inline ipmi_exporter configuration.
	// Mutually exclusive with ConfigFile.
	IPMIConfig util.RawYAML `yaml:"ipmi_config,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "ipmi_exporter"
}

// InstanceKey returns the hostname of the first target or default.
func (c *Config) InstanceKey(defaultKey string) (string, error) {
	if c.Local.Enabled {
		return "localhost", nil
	}
	if len(c.Targets) == 1 {
		return c.Targets[0].Target, nil
	}
	return defaultKey, nil
}

// NewIntegration creates a new ipmi_exporter integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("ipmi"))
}

// New creates a new ipmi_exporter integration.
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	if !c.Local.Enabled && len(c.Targets) == 0 {
		return nil, fmt.Errorf("either local IPMI collection must be enabled or at least one remote target must be configured")
	}

	// Validate remote targets
	for _, target := range c.Targets {
		if target.Name == "" || target.Target == "" {
			return nil, fmt.Errorf("IPMI target must have both name and target fields set")
		}
	}

	return &Integration{
		cfg: c,
		log: log,
	}, nil
}

// Integration is the ipmi_exporter integration. It scrapes metrics from IPMI devices.
type Integration struct {
	cfg *Config
	log log.Logger
}

// MetricsHandler implements Integration.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	// Create a prometheus registry for IPMI metrics
	registry := prometheus.NewRegistry()

	// Register the IPMI collector for hardware monitoring
	// Uses github.com/bougou/go-ipmi library for native Go IPMI collection
	collector := newIPMICollector(i.cfg, i.log)
	registry.MustRegister(collector)

	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), nil
}

// Run satisfies Integration.Run.
func (i *Integration) Run(ctx context.Context) error {
	// Wait for context cancellation
	<-ctx.Done()
	return ctx.Err()
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *Integration) ScrapeConfigs() []config_integrations.ScrapeConfig {
	var res []config_integrations.ScrapeConfig

	// Add local IPMI scrape config if enabled
	if i.cfg.Local.Enabled {
		scrapeConfig := config_integrations.ScrapeConfig{
			JobName:     i.cfg.Name() + "/local",
			MetricsPath: "/metrics",
		}

		// Add module parameter if specified
		if i.cfg.Local.Module != "" {
			queryParams := url.Values{}
			queryParams.Add("module", i.cfg.Local.Module)
			scrapeConfig.QueryParams = queryParams
		}

		res = append(res, scrapeConfig)
	}

	// Add remote target scrape configs
	for _, target := range i.cfg.Targets {
		queryParams := url.Values{}
		queryParams.Add("target", target.Target)
		if target.Module != "" {
			queryParams.Add("module", target.Module)
		}

		res = append(res, config_integrations.ScrapeConfig{
			JobName:     i.cfg.Name() + "/" + target.Name,
			MetricsPath: "/metrics",
			QueryParams: queryParams,
		})
	}

	return res
}

// ipmiCollector implements prometheus.Collector for IPMI metrics
type ipmiCollector struct {
	cfg *Config
	log log.Logger

	// Metric descriptors
	up                 *prometheus.Desc
	temperatureCelsius *prometheus.Desc
	fanSpeedRPM        *prometheus.Desc
	voltageVolts       *prometheus.Desc
	powerWatts         *prometheus.Desc
	currentAmperes     *prometheus.Desc
	sensorState        *prometheus.Desc
	collectorInfo      *prometheus.Desc
}

func newIPMICollector(cfg *Config, logger log.Logger) *ipmiCollector {
	return &ipmiCollector{
		cfg: cfg,
		log: logger,
		up: prometheus.NewDesc(
			"ipmi_up",
			"IPMI device is reachable and responding (1=up, 0=down)",
			[]string{"target"},
			nil,
		),
		temperatureCelsius: prometheus.NewDesc(
			"ipmi_temperature_celsius",
			"Temperature sensor reading in degrees Celsius",
			[]string{"target", "sensor", "id"},
			nil,
		),
		fanSpeedRPM: prometheus.NewDesc(
			"ipmi_fan_speed_rpm",
			"Fan speed sensor reading in RPM",
			[]string{"target", "sensor", "id"},
			nil,
		),
		voltageVolts: prometheus.NewDesc(
			"ipmi_voltage_volts",
			"Voltage sensor reading in volts",
			[]string{"target", "sensor", "id"},
			nil,
		),
		powerWatts: prometheus.NewDesc(
			"ipmi_power_watts",
			"Power sensor reading in watts",
			[]string{"target", "sensor", "id"},
			nil,
		),
		currentAmperes: prometheus.NewDesc(
			"ipmi_current_amperes",
			"Current sensor reading in amperes",
			[]string{"target", "sensor", "id"},
			nil,
		),
		sensorState: prometheus.NewDesc(
			"ipmi_sensor_state",
			"Sensor state (0=nominal, 1=warning, 2=critical, 3=not available)",
			[]string{"target", "sensor", "id"},
			nil,
		),
		collectorInfo: prometheus.NewDesc(
			"ipmi_collector_info",
			"Information about the IPMI collector configuration",
			[]string{"version", "mode"},
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *ipmiCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.temperatureCelsius
	ch <- c.fanSpeedRPM
	ch <- c.voltageVolts
	ch <- c.powerWatts
	ch <- c.currentAmperes
	ch <- c.sensorState
	ch <- c.collectorInfo
}

// Collect implements prometheus.Collector
func (c *ipmiCollector) Collect(ch chan<- prometheus.Metric) {
	// Send collector info
	mode := "remote"
	if c.cfg.Local.Enabled {
		mode = "local"
	}
	ch <- prometheus.MustNewConstMetric(
		c.collectorInfo,
		prometheus.GaugeValue,
		1,
		"1.0.0", // version
		mode,
	)

	// Collect from local IPMI if enabled
	if c.cfg.Local.Enabled {
		c.collectLocal(ch)
	}

	// Collect from remote targets
	for _, target := range c.cfg.Targets {
		c.collectRemote(ch, target)
	}
}

func (c *ipmiCollector) collectLocal(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	// Create local IPMI client (uses OpenIPMI driver for local access)
	client, err := ipmi.NewOpenClient()
	if err != nil {
		level.Error(c.log).Log("msg", "Failed to create local IPMI client", "error", err)
		ch <- prometheus.MustNewConstMetric(
			c.up,
			prometheus.GaugeValue,
			0,
			"localhost",
		)
		return
	}

	// Attempt connection
	if err := client.Connect(ctx); err != nil {
		level.Error(c.log).Log("msg", "Failed to connect to local IPMI device", "error", err)
		ch <- prometheus.MustNewConstMetric(
			c.up,
			prometheus.GaugeValue,
			0,
			"localhost",
		)
		return
	}
	defer client.Close(ctx)

	// Connection successful
	ch <- prometheus.MustNewConstMetric(
		c.up,
		prometheus.GaugeValue,
		1,
		"localhost",
	)

	// Get all sensors
	sensors, err := client.GetSensors(ctx)
	if err != nil {
		level.Error(c.log).Log("msg", "Failed to get local sensors", "error", err)
		return
	}

	// Iterate through all sensors
	for _, sensor := range sensors {
		// Convert sensor ID to string
		sensorID := fmt.Sprintf("%d", sensor.Number)
		sensorName := strings.TrimSpace(sensor.Name)

		// Emit sensor state based on sensor status
		stateValue := c.convertSensorStatus(sensor.Status())
		ch <- prometheus.MustNewConstMetric(
			c.sensorState,
			prometheus.GaugeValue,
			stateValue,
			"localhost",
			sensorName,
			sensorID,
		)

		// Emit metric based on sensor type/unit
		c.emitSensorMetric(ch, "localhost", sensorName, sensorID, sensor)
	}
}

func (c *ipmiCollector) collectRemote(ch chan<- prometheus.Metric, target IPMITarget) {
	// Create context with timeout from config
	timeout := time.Duration(c.cfg.Timeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	level.Debug(c.log).Log("msg", "Starting IPMI collection", "target", target.Target, "timeout", timeout)

	// Default IPMI port
	port := 623

	// Create IPMI client for remote access
	client, err := ipmi.NewClient(target.Target, port, target.User, string(target.Password))
	if err != nil {
		level.Error(c.log).Log("msg", "Failed to create IPMI client", "target", target.Target, "error", err)
		ch <- prometheus.MustNewConstMetric(
			c.up,
			prometheus.GaugeValue,
			0,
			target.Target,
		)
		return
	}

	// Set client timeout to 80% of total timeout to allow time for processing
	client = client.WithTimeout(timeout * 4 / 5)

	level.Debug(c.log).Log("msg", "Attempting IPMI connection", "target", target.Target, "driver", target.Driver)

	// Attempt connection - use Connect20 for LAN_2_0 or Connect for auto-detect
	var connectErr error
	if target.Driver == "LAN_2_0" || target.Driver == "" {
		connectErr = client.Connect20(ctx)
	} else {
		connectErr = client.Connect15(ctx)
	}

	if connectErr != nil {
		level.Error(c.log).Log("msg", "Failed to connect to IPMI device", "target", target.Target, "driver", target.Driver, "error", connectErr)
		ch <- prometheus.MustNewConstMetric(
			c.up,
			prometheus.GaugeValue,
			0,
			target.Target,
		)
		return
	}
	defer client.Close(ctx)

	level.Debug(c.log).Log("msg", "IPMI connection successful", "target", target.Target)

	// Connection successful
	ch <- prometheus.MustNewConstMetric(
		c.up,
		prometheus.GaugeValue,
		1,
		target.Target,
	)

	// Get all sensors
	level.Debug(c.log).Log("msg", "Fetching sensors from IPMI device", "target", target.Target)

	sensors, err := client.GetSensors(ctx)
	if err != nil {
		// Log the error but still report ipmi_up=1 since we connected successfully
		level.Warn(c.log).Log("msg", "Failed to retrieve sensors (connection succeeded but sensor reading failed)", "target", target.Target, "error", err)
		// Don't return - we already set ipmi_up=1 which is accurate (connection worked)
		return
	}

	level.Info(c.log).Log("msg", "Successfully retrieved sensors", "target", target.Target, "sensor_count", len(sensors))

	// Iterate through all sensors
	for _, sensor := range sensors {
		// Convert sensor ID to string
		sensorID := fmt.Sprintf("%d", sensor.Number)
		sensorName := strings.TrimSpace(sensor.Name)

		// Emit sensor state based on sensor status
		stateValue := c.convertSensorStatus(sensor.Status())
		ch <- prometheus.MustNewConstMetric(
			c.sensorState,
			prometheus.GaugeValue,
			stateValue,
			target.Target,
			sensorName,
			sensorID,
		)

		// Emit metric based on sensor type/unit
		c.emitSensorMetric(ch, target.Target, sensorName, sensorID, sensor)
	}
}

// convertSensorStatus converts IPMI sensor status string to numeric value
func (c *ipmiCollector) convertSensorStatus(status string) float64 {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "ok", "nominal":
		return 0
	case "warning", "warn", "nc":
		return 1
	case "critical", "crit", "cr":
		return 2
	case "n/a", "na", "unavailable":
		return 3
	default:
		return 0
	}
}

// emitSensorMetric emits the appropriate metric based on sensor unit
func (c *ipmiCollector) emitSensorMetric(ch chan<- prometheus.Metric, target, sensorName, sensorID string, sensor *ipmi.Sensor) {
	// Skip sensors without analog readings
	if !sensor.HasAnalogReading {
		return
	}

	// Use the already-converted value from the sensor
	value := sensor.Value

	// Determine metric type based on sensor base unit
	switch sensor.SensorUnit.BaseUnit {
	case ipmi.SensorUnitType_DegreesC:
		ch <- prometheus.MustNewConstMetric(
			c.temperatureCelsius,
			prometheus.GaugeValue,
			value,
			target,
			sensorName,
			sensorID,
		)
	case ipmi.SensorUnitType_RPM:
		ch <- prometheus.MustNewConstMetric(
			c.fanSpeedRPM,
			prometheus.GaugeValue,
			value,
			target,
			sensorName,
			sensorID,
		)
	case ipmi.SensorUnitType_Volts:
		ch <- prometheus.MustNewConstMetric(
			c.voltageVolts,
			prometheus.GaugeValue,
			value,
			target,
			sensorName,
			sensorID,
		)
	case ipmi.SensorUnitType_Watts:
		ch <- prometheus.MustNewConstMetric(
			c.powerWatts,
			prometheus.GaugeValue,
			value,
			target,
			sensorName,
			sensorID,
		)
	case ipmi.SensorUnitType_Amps:
		ch <- prometheus.MustNewConstMetric(
			c.currentAmperes,
			prometheus.GaugeValue,
			value,
			target,
			sensorName,
			sensorID,
		)
	}
}
