package gcp_exporter

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/multierror"
	"github.com/prometheus-community/stackdriver_exporter/collectors"
	"github.com/prometheus-community/stackdriver_exporter/delta"
	"github.com/prometheus-community/stackdriver_exporter/utils"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/monitoring/v3"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v2"

	"github.com/grafana/agent/internal/static/integrations"
	integrations_v2 "github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/grafana/agent/internal/static/integrations/v2/metricsutils"
)

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("gcp"))
}

type Config struct {
	ProjectIDs            []string      `yaml:"project_ids"`
	MetricPrefixes        []string      `yaml:"metrics_prefixes"`
	ExtraFilters          []string      `yaml:"extra_filters"`
	RequestInterval       time.Duration `yaml:"request_interval"`
	RequestOffset         time.Duration `yaml:"request_offset"`
	IngestDelay           bool          `yaml:"ingest_delay"`
	DropDelegatedProjects bool          `yaml:"drop_delegated_projects"`
	ClientTimeout         time.Duration `yaml:"gcp_client_timeout"`
}

var DefaultConfig = Config{
	ClientTimeout:         15 * time.Second,
	RequestInterval:       5 * time.Minute,
	RequestOffset:         0,
	IngestDelay:           false,
	DropDelegatedProjects: false,
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

func (c *Config) Name() string {
	return "gcp_exporter"
}

func (c *Config) InstanceKey(_ string) (string, error) {
	// We use a hash of the config so our key is unique when leveraged with v2
	// The config itself doesn't have anything which can uniquely identify it.
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:]), nil
}

func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	svc, err := createMonitoringService(context.Background(), c.ClientTimeout)
	if err != nil {
		return nil, err
	}

	var gcpCollectors []prometheus.Collector
	for _, projectID := range c.ProjectIDs {
		monitoringCollector, err := collectors.NewMonitoringCollector(
			projectID,
			svc,
			collectors.MonitoringCollectorOptions{
				MetricTypePrefixes:    c.MetricPrefixes,
				ExtraFilters:          parseMetricExtraFilters(c.ExtraFilters),
				RequestInterval:       c.RequestInterval,
				RequestOffset:         c.RequestOffset,
				IngestDelay:           c.IngestDelay,
				DropDelegatedProjects: c.DropDelegatedProjects,

				// If FillMissingLabels ensures all metrics with the same name have the same label set. It's not often
				// that GCP metrics have different labels but if it happens the prom registry will panic.
				FillMissingLabels: true,

				// If AggregateDeltas is disabled the data produced is not useful at all. See https://github.com/prometheus-community/stackdriver_exporter#what-to-know-about-aggregating-delta-metrics
				// for more info
				AggregateDeltas: true,
			},
			l,
			delta.NewInMemoryCounterStore(l, 30*time.Minute),
			delta.NewInMemoryHistogramStore(l, 30*time.Minute),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create monitoring collector: %w", err)
		}
		gcpCollectors = append(gcpCollectors, monitoringCollector)
	}

	return integrations.NewCollectorIntegration(
		c.Name(), integrations.WithCollectors(gcpCollectors...),
	), nil
}

func (c *Config) Validate() error {
	configErrors := multierror.MultiError{}

	if c.ProjectIDs == nil || len(c.ProjectIDs) == 0 {
		configErrors.Add(errors.New("no project_ids defined"))
	}

	if c.MetricPrefixes == nil || len(c.MetricPrefixes) == 0 {
		configErrors.Add(errors.New("at least 1 metrics_prefixes is required"))
	}

	if len(c.ExtraFilters) > 0 {
		filterPrefixToFilter := map[string][]string{}
		for _, filter := range c.ExtraFilters {
			splitFilter := strings.Split(filter, ":")
			if len(splitFilter) <= 1 {
				configErrors.Add(fmt.Errorf("%s is an invalid filter a filter must be of the form <metric_type>:<filter_expression>", filter))
				continue
			}
			filterPrefix := splitFilter[0]
			if _, exists := filterPrefixToFilter[filterPrefix]; !exists {
				filterPrefixToFilter[filterPrefix] = []string{}
			}
			filterPrefixToFilter[filterPrefix] = append(filterPrefixToFilter[filterPrefix], filter)
		}

		for filterPrefix, filters := range filterPrefixToFilter {
			validFilterPrefix := false
			for _, metricPrefix := range c.MetricPrefixes {
				if strings.HasPrefix(metricPrefix, filterPrefix) {
					validFilterPrefix = true
					break
				}
			}
			if !validFilterPrefix {
				configErrors.Add(fmt.Errorf("no metric_prefixes started with %s which means the extra_filters %s will not have any effect", filterPrefix, strings.Join(filters, ",")))
			}
		}
	}

	return configErrors.Err()
}

func createMonitoringService(ctx context.Context, httpTimeout time.Duration) (*monitoring.Service, error) {
	googleClient, err := google.DefaultClient(ctx, monitoring.MonitoringReadScope)
	if err != nil {
		return nil, fmt.Errorf("error creating Google client: %v", err)
	}

	googleClient.Timeout = httpTimeout
	googleClient.Transport = rehttp.NewTransport(
		googleClient.Transport,
		rehttp.RetryAll(
			rehttp.RetryMaxRetries(4),
			rehttp.RetryStatuses(http.StatusServiceUnavailable)),
		rehttp.ExpJitterDelay(time.Second, 5*time.Second),
	)

	monitoringService, err := monitoring.NewService(ctx,
		option.WithHTTPClient(googleClient),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Google Stackdriver Monitoring service: %v", err)
	}

	return monitoringService, nil
}

func parseMetricExtraFilters(filters []string) []collectors.MetricFilter {
	var extraFilters []collectors.MetricFilter
	for _, ef := range filters {
		efPrefix, efModifier := utils.GetExtraFilterModifiers(ef, ":")
		if efPrefix != "" {
			extraFilter := collectors.MetricFilter{
				Prefix:   efPrefix,
				Modifier: efModifier,
			}
			extraFilters = append(extraFilters, extraFilter)
		}
	}
	return extraFilters
}
