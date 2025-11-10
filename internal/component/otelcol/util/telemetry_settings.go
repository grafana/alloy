package util

import (
	"github.com/grafana/alloy/internal/build"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/service/telemetry"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"

	"context"
	"os"
)

var cachedResource *pcommon.Resource

// GetTelemetrySettingsResource returns a resource describing the telemetry
// settings of the OpenTelemetry Collector instance.

func GetTelemetrySettingsResource() (pcommon.Resource, error) {
	if cachedResource != nil {
		return *cachedResource, nil
	}

	telemetrySettings := telemetry.Settings{BuildInfo: GetBuildInfo()}

	fact := otelconftelemetry.NewFactory()
	resource, err := fact.CreateResource(context.Background(), telemetrySettings, fact.CreateDefaultConfig())
	if err != nil {
		return pcommon.Resource{}, err
	}
	cachedResource = &resource
	return resource, nil
}

func GetBuildInfo() otelcomponent.BuildInfo {
	return otelcomponent.BuildInfo{
		Command:     os.Args[0],
		Description: "Grafana Alloy",
		Version:     build.Version,
	}
}
