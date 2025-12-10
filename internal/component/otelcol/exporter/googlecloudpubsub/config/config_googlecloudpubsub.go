package googlecloudpubsubconfig

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"
)

type GoogleCloudPubSubWatermarkArguments struct {
	Behavior     string        `alloy:"behavior,attr,optional"`
	AllowedDrift time.Duration `alloy:"allowed_drift,attr,optional"`
}

func (args *GoogleCloudPubSubWatermarkArguments) Convert() googlecloudpubsubexporter.WatermarkConfig {
	if args == nil {
		return googlecloudpubsubexporter.WatermarkConfig{}
	}

	return googlecloudpubsubexporter.WatermarkConfig{
		Behavior:     args.Behavior,
		AllowedDrift: args.AllowedDrift,
	}
}

func (args *GoogleCloudPubSubWatermarkArguments) SetToDefault() {
	*args = GoogleCloudPubSubWatermarkArguments{}
}

type GoogleCloudPubSubOrderingConfigArguments struct {
	Enabled                 bool   `alloy:"enabled,attr,optional"`
	FromResourceAttribute   string `alloy:"from_resource_attribute,attr,optional"`
	RemoveResourceAttribute bool   `alloy:"remove_resource_attribute,attr,optional"`
}

func (args *GoogleCloudPubSubOrderingConfigArguments) Convert() googlecloudpubsubexporter.OrderingConfig {
	if args == nil {
		return googlecloudpubsubexporter.OrderingConfig{}
	}

	return googlecloudpubsubexporter.OrderingConfig{
		Enabled:                 args.Enabled,
		FromResourceAttribute:   args.FromResourceAttribute,
		RemoveResourceAttribute: args.RemoveResourceAttribute,
	}
}

func (args *GoogleCloudPubSubOrderingConfigArguments) SetToDefault() {
	*args = GoogleCloudPubSubOrderingConfigArguments{}
}
