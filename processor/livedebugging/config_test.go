// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package livedebugging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	valid := func() *Config { return createDefaultConfig().(*Config) }

	require.NoError(t, valid().Validate())

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"zero max attribution series", func(c *Config) { c.MaxAttributionSeries = 0 }, "max_attribution_series"},
		{"zero max span name series", func(c *Config) { c.MaxSpanNameSeries = 0 }, "max_span_name_series"},
		{"empty namespace key", func(c *Config) { c.NamespaceAttributeKey = "" }, "namespace_attribute_key"},
		{"empty service name key", func(c *Config) { c.ServiceNameAttributeKey = "" }, "service_name_attribute_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestConfig_Validate_StreamDisabledSkipsStreamChecks(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Stream.MaxConcurrentStreams = 0 // would be invalid if the stream were enabled
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_StreamEnabled(t *testing.T) {
	valid := func() *Config {
		cfg := createDefaultConfig().(*Config)
		cfg.Stream.Enabled = true
		return cfg
	}
	require.NoError(t, valid().Validate())

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"empty endpoint", func(c *Config) { c.Stream.NetAddr.Endpoint = "" }, "stream::endpoint"},
		{"zero default duration", func(c *Config) { c.Stream.DefaultStreamDuration = 0 }, "stream::default_stream_duration"},
		{"zero max duration", func(c *Config) { c.Stream.MaxStreamDuration = 0 }, "stream::max_stream_duration"},
		{"default exceeds max", func(c *Config) { c.Stream.DefaultStreamDuration = 2 * c.Stream.MaxStreamDuration }, "must not exceed"},
		{"zero max concurrent streams", func(c *Config) { c.Stream.MaxConcurrentStreams = 0 }, "stream::max_concurrent_streams"},
		{"zero max spans per second", func(c *Config) { c.Stream.MaxSpansPerSecond = 0 }, "stream::max_spans_per_second"},
		{"zero stream buffer size", func(c *Config) { c.Stream.StreamBufferSize = 0 }, "stream::stream_buffer_size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 1000, cfg.MaxAttributionSeries)
	assert.Equal(t, 1000, cfg.MaxSpanNameSeries)
	assert.Equal(t, "k8s.namespace.name", cfg.NamespaceAttributeKey)
	assert.Equal(t, "service.name", cfg.ServiceNameAttributeKey)
	require.NoError(t, cfg.Validate())
}
