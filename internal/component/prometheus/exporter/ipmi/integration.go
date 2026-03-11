package ipmi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	goipmi "github.com/bougou/go-ipmi"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/alloy/internal/static/integrations"
	config_integrations "github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/grafana/alloy/internal/util"
)

// Config controls the ipmi_exporter integration.
type Config struct {
	Local      LocalConfig  `yaml:"local,omitempty"`
	Targets    []IPMITarget `yaml:"targets,omitempty"`
	Timeout    int64        `yaml:"timeout,omitempty"`
	ConfigFile string       `yaml:"config_file,omitempty"`
	IPMIConfig util.RawYAML `yaml:"ipmi_config,omitempty"`
}

var _ integrations.Config = (*Config)(nil)

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
	if !c.Local.Enabled && len(c.Targets) == 0 {
		return nil, fmt.Errorf("either local IPMI collection must be enabled or at least one remote target must be configured")
	}

	for _, target := range c.Targets {
		if target.Name == "" || target.Target == "" {
			return nil, fmt.Errorf("IPMI target must have both name and target fields set")
		}
	}

	return &integration{
		cfg: c,
		log: l,
	}, nil
}

// integration is the ipmi_exporter integration.
type integration struct {
	cfg *Config
	log log.Logger
}

// MetricsHandler implements Integration.
func (i *integration) MetricsHandler() (http.Handler, error) {
	registry := prometheus.NewRegistry()
	collector := newIPMICollector(i.cfg, i.log)
	registry.MustRegister(collector)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), nil
}

// Run satisfies Integration.Run.
func (i *integration) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *integration) ScrapeConfigs() []config_integrations.ScrapeConfig {
	var res []config_integrations.ScrapeConfig

	if i.cfg.Local.Enabled {
		scrapeConfig := config_integrations.ScrapeConfig{
			JobName:     i.cfg.Name() + "/local",
			MetricsPath: "/metrics",
		}

		if i.cfg.Local.Module != "" {
			queryParams := url.Values{}
			queryParams.Add("module", i.cfg.Local.Module)
			scrapeConfig.QueryParams = queryParams
		}

		res = append(res, scrapeConfig)
	}

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
	mode := "remote"
	if c.cfg.Local.Enabled {
		mode = "local"
	}
	ch <- prometheus.MustNewConstMetric(
		c.collectorInfo,
		prometheus.GaugeValue,
		1,
		"1.0.0",
		mode,
	)

	if c.cfg.Local.Enabled {
		c.collectLocal(ch)
	}

	for _, target := range c.cfg.Targets {
		c.collectRemote(ch, target)
	}
}

func (c *ipmiCollector) collectLocal(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	client, err := goipmi.NewOpenClient()
	if err != nil {
		level.Error(c.log).Log("msg", "Failed to create local IPMI client", "error", err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, "localhost")
		return
	}

	if err := client.Connect(ctx); err != nil {
		level.Error(c.log).Log("msg", "Failed to connect to local IPMI device", "error", err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, "localhost")
		return
	}
	defer client.Close(ctx)

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, "localhost")

	sensors, err := client.GetSensors(ctx)
	if err != nil {
		level.Error(c.log).Log("msg", "Failed to get local sensors", "error", err)
		return
	}

	for _, sensor := range sensors {
		sensorID := fmt.Sprintf("%d", sensor.Number)
		sensorName := strings.TrimSpace(sensor.Name)

		stateValue := c.convertSensorStatus(sensor.Status())
		ch <- prometheus.MustNewConstMetric(c.sensorState, prometheus.GaugeValue, stateValue, "localhost", sensorName, sensorID)

		c.emitSensorMetric(ch, "localhost", sensorName, sensorID, sensor)
	}
}

func (c *ipmiCollector) collectRemote(ch chan<- prometheus.Metric, target IPMITarget) {
	timeout := time.Duration(c.cfg.Timeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	level.Debug(c.log).Log("msg", "Starting IPMI collection", "target", target.Target, "timeout", timeout)

	client, err := goipmi.NewClient(target.Target, 623, target.User, string(target.Password))
	if err != nil {
		level.Error(c.log).Log("msg", "Failed to create IPMI client", "target", target.Target, "error", err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, target.Target)
		return
	}

	client = client.WithTimeout(timeout * 4 / 5)

	level.Debug(c.log).Log("msg", "Attempting IPMI connection", "target", target.Target, "driver", target.Driver)

	var connectErr error
	if target.Driver == "LAN_2_0" || target.Driver == "" {
		connectErr = client.Connect20(ctx)
	} else {
		connectErr = client.Connect15(ctx)
	}

	if connectErr != nil {
		level.Error(c.log).Log("msg", "Failed to connect to IPMI device", "target", target.Target, "driver", target.Driver, "error", connectErr)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, target.Target)
		return
	}
	defer client.Close(ctx)

	level.Debug(c.log).Log("msg", "IPMI connection successful", "target", target.Target)
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, target.Target)

	level.Debug(c.log).Log("msg", "Fetching sensors from IPMI device", "target", target.Target)

	sensors, err := client.GetSensors(ctx)
	if err != nil {
		level.Warn(c.log).Log("msg", "Failed to retrieve sensors (connection succeeded but sensor reading failed)", "target", target.Target, "error", err)
		return
	}

	level.Info(c.log).Log("msg", "Successfully retrieved sensors", "target", target.Target, "sensor_count", len(sensors))

	for _, sensor := range sensors {
		sensorID := fmt.Sprintf("%d", sensor.Number)
		sensorName := strings.TrimSpace(sensor.Name)

		stateValue := c.convertSensorStatus(sensor.Status())
		ch <- prometheus.MustNewConstMetric(c.sensorState, prometheus.GaugeValue, stateValue, target.Target, sensorName, sensorID)

		c.emitSensorMetric(ch, target.Target, sensorName, sensorID, sensor)
	}
}

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

func (c *ipmiCollector) emitSensorMetric(ch chan<- prometheus.Metric, target, sensorName, sensorID string, sensor *goipmi.Sensor) {
	if !sensor.HasAnalogReading {
		return
	}

	value := sensor.Value

	switch sensor.SensorUnit.BaseUnit {
	case goipmi.SensorUnitType_DegreesC:
		ch <- prometheus.MustNewConstMetric(c.temperatureCelsius, prometheus.GaugeValue, value, target, sensorName, sensorID)
	case goipmi.SensorUnitType_RPM:
		ch <- prometheus.MustNewConstMetric(c.fanSpeedRPM, prometheus.GaugeValue, value, target, sensorName, sensorID)
	case goipmi.SensorUnitType_Volts:
		ch <- prometheus.MustNewConstMetric(c.voltageVolts, prometheus.GaugeValue, value, target, sensorName, sensorID)
	case goipmi.SensorUnitType_Watts:
		ch <- prometheus.MustNewConstMetric(c.powerWatts, prometheus.GaugeValue, value, target, sensorName, sensorID)
	case goipmi.SensorUnitType_Amps:
		ch <- prometheus.MustNewConstMetric(c.currentAmperes, prometheus.GaugeValue, value, target, sensorName, sensorID)
	}
}
