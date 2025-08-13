package azure_exporter_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	"github.com/webdevops/azure-metrics-exporter/metrics"

	"github.com/grafana/alloy/internal/static/integrations/azure_exporter"
)

func TestConfig_ToScrapeSettings(t *testing.T) {
	baseConfig := azure_exporter.Config{
		Subscriptions:            []string{"subscriptionA"},
		ResourceType:             "resourceType",
		ResourceGraphQueryFilter: "filter_me",
		Metrics:                  []string{"MetricA"},
		MetricAggregations:       []string{"MiNimUm"},
		Timespan:                 "timespan_me",
		Interval:                 "interval_me",
		IncludedResourceTags:     []string{"tag_me"},
		MetricNamespace:          "namespace_me",
		MetricNameTemplate:       "name_template_me",
		MetricHelpTemplate:       "help_template_me",
		AzureCloudEnvironment:    "azurecloud",
	}
	baseSettings := metrics.RequestMetricSettings{
		Subscriptions:   []string{"subscriptionA"},
		ResourceType:    "resourceType",
		Filter:          "filter_me",
		Timespan:        "timespan_me",
		Interval:        to.Ptr("interval_me"),
		Metrics:         []string{"MetricA"},
		MetricNamespace: "namespace_me",
		Aggregations:    []string{"MiNimUm"},
		MetricTemplate:  "name_template_me",
		HelpTemplate:    "help_template_me",

		// Should not be set
		Name:          "",
		MetricTop:     nil,
		MetricFilter:  "",
		MetricOrderBy: "",
		Cache:         nil,
	}

	baseConfigValid := t.Run("maps expected fields", func(t *testing.T) {
		settings, err := baseConfig.ToScrapeSettings()
		require.NoError(t, err)
		require.Equal(t, &baseSettings, settings)
	})
	if !baseConfigValid {
		return
	}

	tests := []struct {
		name               string
		configModifier     func(azure_exporter.Config) azure_exporter.Config
		toExpectedSettings func(metrics.RequestMetricSettings) metrics.RequestMetricSettings
	}{
		{
			name: "can set a metric filter for a single dimension",
			configModifier: func(config azure_exporter.Config) azure_exporter.Config {
				config.IncludedDimensions = []string{"dimension1"}
				return config
			},
			toExpectedSettings: func(settings metrics.RequestMetricSettings) metrics.RequestMetricSettings {
				settings.MetricFilter = "dimension1 eq '*'"
				settings.MetricTop = to.Ptr[int32](100_000_000)
				return settings
			},
		},
		{
			name: "can set a metric filter for a multiple dimensions",
			configModifier: func(config azure_exporter.Config) azure_exporter.Config {
				config.IncludedDimensions = []string{"dimension1", "dimension2", "dimension3"}
				return config
			},
			toExpectedSettings: func(settings metrics.RequestMetricSettings) metrics.RequestMetricSettings {
				settings.MetricFilter = "dimension1 eq '*' and dimension2 eq '*' and dimension3 eq '*'"
				settings.MetricTop = to.Ptr[int32](100_000_000)
				return settings
			},
		},
		{
			name: "sets regions and top when using regions with no dimensions",
			configModifier: func(config azure_exporter.Config) azure_exporter.Config {
				config.Regions = []string{"uswest", "useast"}
				return config
			},
			toExpectedSettings: func(settings metrics.RequestMetricSettings) metrics.RequestMetricSettings {
				settings.MetricTop = to.Ptr[int32](100_000_000)
				settings.Regions = []string{"uswest", "useast"}
				return settings
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullConfig := tt.configModifier(baseConfig)
			expectedSettings := tt.toExpectedSettings(baseSettings)
			settings, err := fullConfig.ToScrapeSettings()
			require.NoError(t, err)
			require.Equal(t, &expectedSettings, settings)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	baseConfig := azure_exporter.Config{
		Subscriptions:         []string{"subscriptionA"},
		ResourceType:          "resourceType",
		Metrics:               []string{"MetricA"},
		AzureCloudEnvironment: "azurecloud",
		Interval:              "PT1M",
		Timespan:              "PT5M",
	}

	baseConfigValid := t.Run("Base Config is Valid", func(t *testing.T) {
		err := baseConfig.Validate()
		require.NoError(t, err, "Base config was not valid but needs to be for these tests")
	})
	if !baseConfigValid {
		return
	}

	tests := []struct {
		name            string
		toInvalidConfig func(azure_exporter.Config) azure_exporter.Config
	}{
		{
			name: "nil Subscriptions",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.Subscriptions = nil
				return config
			},
		},
		{
			name: "empty Subscriptions",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.Subscriptions = []string{}
				return config
			},
		},
		{
			name: "empty ResourceType",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.ResourceType = ""
				return config
			},
		},
		{
			name: "nil metrics",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.Metrics = nil
				return config
			},
		},
		{
			name: "empty metrics",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.Metrics = []string{}
				return config
			},
		},
		{
			name: "invalid aggregation",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.MetricAggregations = []string{"I'm Invalid"}
				return config
			},
		},
		{
			name: "invalid azure_cloud_environment",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.AzureCloudEnvironment = "Not Real"
				return config
			},
		},
		{
			name: "includes Regions and ResourceGraphQueryFilter",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.ResourceGraphQueryFilter = "filter the resources"
				config.Regions = []string{"uswest", "useast"}
				return config
			},
		},
		{
			name: "invalid interval",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.Interval = "Not a valid interval"
				return config
			},
		},
		{
			name: "invalid timespan",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.Timespan = "Not a valid timespan"
				return config
			},
		},
		{
			name: "interval larger than timespan",
			toInvalidConfig: func(config azure_exporter.Config) azure_exporter.Config {
				config.Timespan = "PT1M"
				config.Interval = "PT5M"
				return config
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalidConfig := tt.toInvalidConfig(baseConfig)
			err := invalidConfig.Validate()
			require.Error(t, err)
		})
	}
}

func TestMergeConfigWithQueryParams_MapsAllExpectedFieldsByYamlNameFromConfig(t *testing.T) {
	// We want to be sure all expected fields are mappable by the yaml name and reflect allows us to do that programmatically
	thing := reflect.TypeOf(azure_exporter.Config{})
	var mappableFields []reflect.StructField
	for i := 0; i < thing.NumField(); i++ {
		field := thing.Field(i)
		// Not available to be mapped via query param
		if field.Name == "AzureCloudEnvironment" {
			continue
		}

		mappableFields = append(mappableFields, field)
	}

	for _, mappableField := range mappableFields {
		yamlFieldName := mappableField.Tag.Get("yaml")
		t.Run(fmt.Sprintf("Can map %s from query param", yamlFieldName), func(t *testing.T) {
			urlParams := map[string][]string{}
			var fieldValue any

			switch mappableField.Type.String() {
			case "string":
				value := "fake string 1"
				urlParams[yamlFieldName] = []string{value}
				fieldValue = value
			case "[]string":
				value := []string{"fake string 1", "fake string 2"}
				fieldValue = value
				urlParams[yamlFieldName] = value
			case "bool":
				urlParams[yamlFieldName] = []string{"false"}
				fieldValue = false
			default:
				t.Fatalf("Attempting to map %s, discovered unexpected type %s", mappableField.Name, mappableField.Type.String())
			}

			expectedConfig := &azure_exporter.Config{}
			reflect.ValueOf(expectedConfig).Elem().FieldByName(mappableField.Name).Set(reflect.ValueOf(fieldValue))

			actualConfig, err := azure_exporter.MergeConfigWithQueryParams(azure_exporter.Config{}, urlParams)
			require.NoError(t, err)
			require.Equal(t, *expectedConfig, actualConfig)
		})
	}
}
