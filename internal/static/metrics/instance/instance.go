// Package instance provides a mini Prometheus scraper and remote_writer.
package instance

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage/remote"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/useragent"
)

func init() {
	remote.UserAgent = useragent.Get()
	scrape.UserAgent = useragent.Get()

	// default remote_write send_exemplars to true
	config.DefaultRemoteWriteConfig.SendExemplars = true
	// default remote_write retry_on_http_429 to true
	config.DefaultRemoteWriteConfig.QueueConfig.RetryOnRateLimit = true
}

// Default configuration values
var (
	DefaultConfig = Config{
		HostFilter:           false,
		WALTruncateFrequency: 60 * time.Minute,
		MinWALTime:           5 * time.Minute,
		MaxWALTime:           4 * time.Hour,
		RemoteFlushDeadline:  1 * time.Minute,
		WriteStaleOnShutdown: false,
		global:               DefaultGlobalConfig,
	}
)

// Config is a specific agent that runs within the overall Prometheus
// agent. It has its own set of scrape_configs and remote_write rules.
type Config struct {
	Name                     string                      `yaml:"name,omitempty"`
	HostFilter               bool                        `yaml:"host_filter,omitempty"`
	HostFilterRelabelConfigs []*relabel.Config           `yaml:"host_filter_relabel_configs,omitempty"`
	ScrapeConfigs            []*config.ScrapeConfig      `yaml:"scrape_configs,omitempty"`
	RemoteWrite              []*config.RemoteWriteConfig `yaml:"remote_write,omitempty"`

	// How frequently the WAL should be truncated.
	WALTruncateFrequency time.Duration `yaml:"wal_truncate_frequency,omitempty"`

	// Minimum and maximum time series should exist in the WAL for.
	MinWALTime time.Duration `yaml:"min_wal_time,omitempty"`
	MaxWALTime time.Duration `yaml:"max_wal_time,omitempty"`

	RemoteFlushDeadline  time.Duration `yaml:"remote_flush_deadline,omitempty"`
	WriteStaleOnShutdown bool          `yaml:"write_stale_on_shutdown,omitempty"`

	global GlobalConfig `yaml:"-"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// MarshalYAML implements yaml.Marshaler.
func (c Config) MarshalYAML() (any, error) {
	// We want users to be able to marshal instance.Configs directly without
	// *needing* to call instance.MarshalConfig, so we call it internally
	// here and return a map.
	bb, err := MarshalConfig(&c, true)
	if err != nil {
		return nil, err
	}

	// Use a yaml.MapSlice rather than a map[string]interface{} so
	// order of keys is retained compared to just calling MarshalConfig.
	var m yaml.MapSlice
	if err := yaml.Unmarshal(bb, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ApplyDefaults applies default configurations to the configuration to all
// values that have not been changed to their non-zero value. ApplyDefaults
// also validates the config.
//
// The value for global will be saved.
func (c *Config) ApplyDefaults(global GlobalConfig) error {
	c.global = global

	switch {
	case c.Name == "":
		return errors.New("missing instance name")
	case c.WALTruncateFrequency <= 0:
		return errors.New("wal_truncate_frequency must be greater than 0s")
	case c.RemoteFlushDeadline <= 0:
		return errors.New("remote_flush_deadline must be greater than 0s")
	case c.MinWALTime > c.MaxWALTime:
		return errors.New("min_wal_time must be less than max_wal_time")
	}

	jobNames := map[string]struct{}{}
	for _, sc := range c.ScrapeConfigs {
		if sc == nil {
			return fmt.Errorf("empty or null scrape config section")
		}

		// First set the correct scrape interval, then check that the timeout
		// (inferred or explicit) is not greater than that.
		if sc.ScrapeInterval == 0 {
			sc.ScrapeInterval = c.global.Prometheus.ScrapeInterval
		}
		if sc.ScrapeTimeout > sc.ScrapeInterval {
			return fmt.Errorf("scrape timeout greater than scrape interval for scrape config with job name %q", sc.JobName)
		}
		if time.Duration(sc.ScrapeInterval) > c.WALTruncateFrequency {
			return fmt.Errorf("scrape interval greater than wal_truncate_frequency for scrape config with job name %q", sc.JobName)
		}
		if sc.ScrapeTimeout == 0 {
			if c.global.Prometheus.ScrapeTimeout > sc.ScrapeInterval {
				sc.ScrapeTimeout = sc.ScrapeInterval
			} else {
				sc.ScrapeTimeout = c.global.Prometheus.ScrapeTimeout
			}
		}
		if sc.ScrapeProtocols == nil {
			sc.ScrapeProtocols = c.global.Prometheus.ScrapeProtocols
		}
		if sc.MetricNameValidationScheme == model.UnsetValidation {
			sc.MetricNameValidationScheme = c.global.Prometheus.MetricNameValidationScheme
		}
		if sc.MetricNameEscapingScheme == "" {
			sc.MetricNameEscapingScheme = c.global.Prometheus.MetricNameEscapingScheme
		}
		if err := validateScrapeProtocols(sc.ScrapeProtocols); err != nil {
			return fmt.Errorf("invalid scrape protocols provided: %w", err)
		}

		if _, exists := jobNames[sc.JobName]; exists {
			return fmt.Errorf("found multiple scrape configs with job name %q", sc.JobName)
		}
		jobNames[sc.JobName] = struct{}{}
	}

	rwNames := map[string]struct{}{}
	// If the instance remote write is not filled in, then apply the prometheus write config
	if len(c.RemoteWrite) == 0 {
		c.RemoteWrite = c.global.RemoteWrite
	}
	for _, cfg := range c.RemoteWrite {
		if cfg == nil {
			return fmt.Errorf("empty or null remote write config section")
		}
		// Typically Prometheus ignores empty names here, but we need to assign a
		// unique name to the config so we can pull metrics from it when running
		// an instance.
		var generatedName bool
		if cfg.Name == "" {
			hash, err := getHash(cfg)
			if err != nil {
				return err
			}

			// We have to add the name of the instance to ensure that generated metrics
			// are unique across multiple agent instances. The remote write queues currently
			// globally register their metrics so we can't inject labels here.
			cfg.Name = c.Name + "-" + hash[:6]
			generatedName = true
		}
		if _, exists := rwNames[cfg.Name]; exists {
			if generatedName {
				return fmt.Errorf("found two identical remote_write configs")
			}
			return fmt.Errorf("found duplicate remote write configs with name %q", cfg.Name)
		}
		rwNames[cfg.Name] = struct{}{}
	}

	return nil
}

func getHash(data any) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:]), nil
}

// validateScrapeProtocols return errors if we see problems with accept scrape protocols option.
func validateScrapeProtocols(sps []config.ScrapeProtocol) error {
	if len(sps) == 0 {
		return errors.New("scrape_protocols cannot be empty")
	}
	dups := map[string]struct{}{}
	for _, sp := range sps {
		if _, ok := dups[strings.ToLower(string(sp))]; ok {
			return fmt.Errorf("duplicated protocol in scrape_protocols, got %v", sps)
		}
		if err := sp.Validate(); err != nil {
			return fmt.Errorf("scrape_protocols: %w", err)
		}
		dups[strings.ToLower(string(sp))] = struct{}{}
	}
	return nil
}
