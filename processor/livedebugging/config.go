// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package livedebugging

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config configures the live_debugging processor.
type Config struct {
	// MaxAttributionSeries bounds the number of distinct (namespace,
	// service_name) combinations tracked for the largest-trace-size gauge.
	// A new combination beyond this cap is redirected into a fixed
	// overflow bucket instead of growing the tracked set unboundedly.
	MaxAttributionSeries int `mapstructure:"max_attribution_series"`
	// MaxSpanNameSeries bounds the number of distinct (service_name,
	// span_name) combinations tracked for the span-frequency counter.
	MaxSpanNameSeries int `mapstructure:"max_span_name_series"`
	// NamespaceAttributeKey is the resource attribute read as the
	// namespace label for trace-size attribution.
	NamespaceAttributeKey string `mapstructure:"namespace_attribute_key"`
	// ServiceNameAttributeKey is the resource attribute read as the
	// service name label.
	ServiceNameAttributeKey string `mapstructure:"service_name_attribute_key"`

	// Stream optionally exposes a laptop-pullable HTTP stream of whatever
	// traces pass through this processor. Disabled by default: this is
	// the one part of live_debugging that does move raw span data, so it
	// stays an explicit opt-in.
	Stream StreamConfig `mapstructure:"stream"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// StreamConfig configures live_debugging's optional pull-based stream.
type StreamConfig struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// Enabled turns the stream server on. Off by default.
	Enabled bool `mapstructure:"enabled"`
	// DefaultStreamDuration is used when a stream request doesn't specify
	// its own duration.
	DefaultStreamDuration time.Duration `mapstructure:"default_stream_duration"`
	// MaxStreamDuration bounds how long any single stream may run,
	// regardless of what a request asks for.
	MaxStreamDuration time.Duration `mapstructure:"max_stream_duration"`
	// MaxConcurrentStreams bounds how many stream requests can be active
	// at once. A request beyond this limit is rejected with 409.
	MaxConcurrentStreams int `mapstructure:"max_concurrent_streams"`
	// MaxSpansPerSecond is a hard safety cap on how many spans a single
	// stream will emit per second, independent of whatever filtering the
	// pipeline applies upstream of this processor. Spans beyond this
	// budget are dropped, not queued.
	MaxSpansPerSecond int `mapstructure:"max_spans_per_second"`
	// StreamBufferSize bounds the number of trace batches buffered per
	// stream between the pipeline's hot path and the (slower) HTTP
	// writer. A full buffer causes the batch to be dropped rather than
	// blocking the pipeline.
	StreamBufferSize int `mapstructure:"stream_buffer_size"`
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	streamServerConfig := confighttp.NewDefaultServerConfig()
	streamServerConfig.NetAddr.Endpoint = "localhost:8478"
	// The stream response is long-lived; disable the write timeout so the
	// server doesn't close a healthy, actively-draining stream once it
	// runs past an otherwise-reasonable deadline.
	streamServerConfig.WriteTimeout = 0

	return &Config{
		MaxAttributionSeries:    1000,
		MaxSpanNameSeries:       1000,
		NamespaceAttributeKey:   "k8s.namespace.name",
		ServiceNameAttributeKey: "service.name",
		Stream: StreamConfig{
			ServerConfig:          streamServerConfig,
			Enabled:               false,
			DefaultStreamDuration: 10 * time.Second,
			MaxStreamDuration:     2 * time.Hour,
			MaxConcurrentStreams:  1,
			MaxSpansPerSecond:     2000,
			StreamBufferSize:      256,
		},
	}
}

// Validate checks if the processor configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.MaxAttributionSeries <= 0 {
		return errors.New("\"max_attribution_series\" must be positive")
	}
	if cfg.MaxSpanNameSeries <= 0 {
		return errors.New("\"max_span_name_series\" must be positive")
	}
	if cfg.NamespaceAttributeKey == "" {
		return errors.New("\"namespace_attribute_key\" must not be empty")
	}
	if cfg.ServiceNameAttributeKey == "" {
		return errors.New("\"service_name_attribute_key\" must not be empty")
	}
	if !cfg.Stream.Enabled {
		return nil
	}
	if cfg.Stream.NetAddr.Endpoint == "" {
		return errors.New("\"stream::endpoint\" is required when the stream is enabled")
	}
	if cfg.Stream.DefaultStreamDuration <= 0 {
		return errors.New("\"stream::default_stream_duration\" must be positive")
	}
	if cfg.Stream.MaxStreamDuration <= 0 {
		return errors.New("\"stream::max_stream_duration\" must be positive")
	}
	if cfg.Stream.DefaultStreamDuration > cfg.Stream.MaxStreamDuration {
		return errors.New("\"stream::default_stream_duration\" must not exceed \"stream::max_stream_duration\"")
	}
	if cfg.Stream.MaxConcurrentStreams <= 0 {
		return errors.New("\"stream::max_concurrent_streams\" must be positive")
	}
	if cfg.Stream.MaxSpansPerSecond <= 0 {
		return errors.New("\"stream::max_spans_per_second\" must be positive")
	}
	if cfg.Stream.StreamBufferSize <= 0 {
		return errors.New("\"stream::stream_buffer_size\" must be positive")
	}
	return nil
}
