package ssh_exporter

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type SSHClientInterface interface {
	RunCommand(command string) (string, error)
}

type SSHCollector struct {
	logger  log.Logger
	target  Target
	client  SSHClientInterface
	metrics map[string]*prometheus.Desc
}

func NewSSHCollector(logger log.Logger, target Target) (*SSHCollector, error) {
	client, err := NewSSHClient(target)
	if err != nil {
		return nil, err
	}
	client.logger = logger

	collector := &SSHCollector{
		logger:  logger,
		target:  target,
		client:  client,
		metrics: make(map[string]*prometheus.Desc),
	}

	// Initialize metric descriptors for custom metrics
	for _, cm := range target.CustomMetrics {
		// Sort label keys for deterministic descriptor and value ordering
		keys := make([]string, 0, len(cm.Labels))
		for k := range cm.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		desc := prometheus.NewDesc(cm.Name, cm.Help, keys, nil)
		collector.metrics[cm.Name] = desc
	}

	return collector, nil
}

func (c *SSHCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.metrics {
		ch <- desc
	}
}

func (c *SSHCollector) Collect(ch chan<- prometheus.Metric) {
	for _, cm := range c.target.CustomMetrics {
		value, err := c.executeCustomCommand(cm)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to execute custom command", "command", cm.Command, "err", err)
			continue
		}

		level.Debug(c.logger).Log("msg", "executed custom command", "command", cm.Command, "value", value)

		// Collect label values in sorted key order
		labelKeys := make([]string, 0, len(cm.Labels))
		for k := range cm.Labels {
			labelKeys = append(labelKeys, k)
		}
		sort.Strings(labelKeys)
		labelValues := make([]string, 0, len(labelKeys))
		for _, k := range labelKeys {
			labelValues = append(labelValues, cm.Labels[k])
		}

		desc := c.metrics[cm.Name]

		var metric prometheus.Metric
		switch strings.ToLower(cm.Type) {
		case "gauge":
			metric = prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labelValues...)
		case "counter":
			metric = prometheus.MustNewConstMetric(desc, prometheus.CounterValue, value, labelValues...)
		default:
			level.Error(c.logger).Log("msg", "unsupported metric type", "type", cm.Type)
			continue
		}

		ch <- metric
	}
}

func (c *SSHCollector) executeCustomCommand(cm CustomMetric) (float64, error) {
	output, err := c.client.RunCommand(cm.Command)
	if err != nil {
		level.Error(c.logger).Log("msg", "SSH command failed", "command", cm.Command, "err", err)
		return 0, err
	}

	level.Debug(c.logger).Log("msg", "SSH command output", "command", cm.Command, "output", output)

	output = strings.TrimSpace(output)

	if cm.ParseRegex != "" {
		re, err := regexp.Compile(cm.ParseRegex)
		if err != nil {
			return 0, fmt.Errorf("invalid parse regex: %w", err)
		}
		matches := re.FindStringSubmatch(output)
		if len(matches) < 2 {
			return 0, fmt.Errorf("no matches found using regex")
		}
		output = matches[1]
	}

	value, err := strconv.ParseFloat(output, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse output '%s' as float: %w", output, err)
	}

	return value, nil
}
