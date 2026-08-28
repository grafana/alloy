// This code was adapted from the consul service discovery
// package in prometheus: https://github.com/prometheus/prometheus/blob/main/discovery/consul/consul.go
// which is copyrighted: 2015 The Prometheus Authors
// and licensed under the Apache License, Version 2.0 (the "License");

package consulagent

import (
	"errors"
	"strings"
	"time"

	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

// SDConfig is the configuration for Consul service discovery.
type SDConfig struct {
	Server       string        `yaml:"server,omitempty"`
	Token        config.Secret `yaml:"token,omitempty"`
	Datacenter   string        `yaml:"datacenter,omitempty"`
	TagSeparator string        `yaml:"tag_separator,omitempty"`
	Scheme       string        `yaml:"scheme,omitempty"`
	Username     string        `yaml:"username,omitempty"`
	Password     config.Secret `yaml:"password,omitempty"`

	// See https://www.consul.io/docs/internals/consensus.html#consistency-modes,
	// stale reads are a lot cheaper and are a necessity if you have >5k targets.
	AllowStale bool `yaml:"allow_stale"`
	// By default use blocking queries (https://www.consul.io/api/index.html#blocking-queries)
	// but allow users to throttle updates if necessary. This can be useful because of "bugs" like
	// https://github.com/hashicorp/consul/issues/3712 which cause an un-necessary
	// amount of requests on consul.
	RefreshInterval model.Duration `yaml:"refresh_interval,omitempty"`

	// See https://www.consul.io/api/catalog.html#list-services
	// The list of services for which targets are discovered.
	// Defaults to all services if empty.
	Services []string `yaml:"services,omitempty"`
	// A list of tags used to filter instances inside a service. Services must contain all tags in the list.
	ServiceTags []string `yaml:"tags,omitempty"`
	// Desired node metadata.
	NodeMeta map[string]string `yaml:"node_meta,omitempty"`

	TLSConfig config.TLSConfig `yaml:"tls_config,omitempty"`
}

// defaultSDConfig is the default Consul SD configuration.
var defaultSDConfig = SDConfig{
	TagSeparator:    ",",
	Scheme:          "http",
	Server:          "localhost:8500",
	AllowStale:      true,
	RefreshInterval: model.Duration(30 * time.Second),
}

func (c *SDConfig) UnmarshalYAML(unmarshal func(any) error) error {
	*c = defaultSDConfig
	type plain SDConfig
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.Server) == "" {
		return errors.New("consulagent SD configuration requires a server address")
	}
	return nil
}
