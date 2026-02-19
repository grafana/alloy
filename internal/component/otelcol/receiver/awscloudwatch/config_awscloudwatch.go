package awscloudwatch

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"
)

var defaultLogGroupLimit = 50

// LogsConfig is the configuration for the logs portion of this receiver
type LogsConfig struct {
	PollInterval        time.Duration `alloy:"poll_interval,attr,optional"`
	MaxEventsPerRequest int           `alloy:"max_events_per_request,attr,optional"`
	Groups              GroupConfig   `alloy:"groups,block,optional"`
	StartFrom           string        `alloy:"start_from,attr,optional"`
}

func (args LogsConfig) Convert() awscloudwatchreceiver.LogsConfig {
	return awscloudwatchreceiver.LogsConfig{
		PollInterval:        args.PollInterval,
		MaxEventsPerRequest: args.MaxEventsPerRequest,
		Groups:              args.Groups.Convert(),
		StartFrom:           args.StartFrom,
	}
}

func (args *LogsConfig) SetToDefault() {
	*args = LogsConfig{
		PollInterval:        time.Minute,
		MaxEventsPerRequest: 1000,
	}
}

// GroupConfig is the configuration for log group collection
type GroupConfig struct {
	AutodiscoverConfig *AutodiscoverConfig `alloy:"autodiscover,block,optional"`
	NamedConfigs       NamedConfigs        `alloy:"named,block,optional"`
}

func (args GroupConfig) Convert() awscloudwatchreceiver.GroupConfig {
	return awscloudwatchreceiver.GroupConfig{
		AutodiscoverConfig: args.AutodiscoverConfig.Convert(),
		NamedConfigs:       args.NamedConfigs.Convert(),
	}
}

type NamedConfigs []NamedConfig

type NamedConfig struct {
	GroupName string    `alloy:"group_name,attr"`
	Prefixes  []*string `alloy:"prefixes,attr,optional"`
	Names     []*string `alloy:"names,attr,optional"`
}

func (args NamedConfigs) Convert() map[string]awscloudwatchreceiver.StreamConfig {
	ret := make(map[string]awscloudwatchreceiver.StreamConfig)
	for _, c := range args {
		ret[c.GroupName] = awscloudwatchreceiver.StreamConfig{
			Prefixes: c.Prefixes,
			Names:    c.Names,
		}
	}
	return ret
}

// AutodiscoverConfig is the configuration for the autodiscovery functionality of log groups
type AutodiscoverConfig struct {
	Prefix                string       `alloy:"prefix,attr,optional"`
	Limit                 *int         `alloy:"limit,attr,optional"`
	Streams               StreamConfig `alloy:"streams,block,optional"`
	AccountIdentifiers    []string     `alloy:"account_identifiers,attr,optional"`
	IncludeLinkedAccounts *bool        `alloy:"include_linked_accounts,attr,optional"`
}

func (args *AutodiscoverConfig) Convert() *awscloudwatchreceiver.AutodiscoverConfig {
	if args == nil {
		return nil
	}
	return &awscloudwatchreceiver.AutodiscoverConfig{
		Prefix:                args.Prefix,
		Limit:                 *args.Limit,
		Streams:               args.Streams.Convert(),
		AccountIdentifiers:    args.AccountIdentifiers,
		IncludeLinkedAccounts: args.IncludeLinkedAccounts,
	}
}

func (args *AutodiscoverConfig) SetToDefault() {
	if args == nil {
		return
	}
	defaultLimit := defaultLogGroupLimit
	*args = AutodiscoverConfig{
		Limit: &defaultLimit,
	}
}

// StreamConfig represents the configuration for the log stream filtering
type StreamConfig struct {
	Prefixes []*string `alloy:"prefixes,attr,optional"`
	Names    []*string `alloy:"names,attr,optional"`
}

func (args StreamConfig) Convert() awscloudwatchreceiver.StreamConfig {
	return awscloudwatchreceiver.StreamConfig{
		Prefixes: args.Prefixes,
		Names:    args.Names,
	}
}
