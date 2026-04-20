package azure_exporter

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/webdevops/azure-metrics-exporter/metrics"
)

func TestGroupResources(t *testing.T) {
	resources := []discoveredResource{
		{ID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1", Location: "eastus", Tags: map[string]string{"env": "prod"}},
		{ID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm2", Location: "eastus", Tags: map[string]string{"env": "prod"}},
		{ID: "/subscriptions/sub1/resourceGroups/rg2/providers/Microsoft.Compute/virtualMachines/vm3", Location: "westus", Tags: map[string]string{"env": "staging"}},
		{ID: "/subscriptions/sub2/resourceGroups/rg3/providers/Microsoft.Compute/virtualMachines/vm4", Location: "eastus", Tags: map[string]string{"env": "prod"}},
	}

	groups := groupResources(resources)

	require.Len(t, groups, 3, "expected 3 groups: (sub1, eastus), (sub1, westus), (sub2, eastus)")

	// Verify group contents
	found := map[string]int{}
	for _, g := range groups {
		key := g.SubscriptionID + ":" + g.Region
		found[key] = len(g.Resources)
	}
	require.Equal(t, 2, found["sub1:eastus"])
	require.Equal(t, 1, found["sub1:westus"])
	require.Equal(t, 1, found["sub2:eastus"])
}

func TestGroupResources_Empty(t *testing.T) {
	groups := groupResources(nil)
	require.Empty(t, groups)
}

func TestGroupResources_InvalidResourceID(t *testing.T) {
	resources := []discoveredResource{
		{ID: "not-a-valid-resource-id", Location: "eastus"},
		{ID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1", Location: "eastus"},
	}

	groups := groupResources(resources)
	require.Len(t, groups, 1, "invalid resource IDs should be skipped")
	require.Len(t, groups[0].Resources, 1)
}

func TestBatchMetricsEndpoint(t *testing.T) {
	tests := []struct {
		region   string
		cloud    string
		expected string
	}{
		{"eastus", "azurecloud", "https://eastus.metrics.monitor.azure.com"},
		{"westeurope", "azurecloud", "https://westeurope.metrics.monitor.azure.com"},
		{"chinaeast", "azurechinacloud", "https://chinaeast.metrics.monitor.azure.cn"},
		{"usgovvirginia", "azuregovernment", "https://usgovvirginia.metrics.monitor.azure.us"},
		{"eastus", "", "https://eastus.metrics.monitor.azure.com"},
	}

	for _, tt := range tests {
		t.Run(tt.region+"_"+tt.cloud, func(t *testing.T) {
			endpoint := batchMetricsEndpoint(tt.region, tt.cloud)
			require.Equal(t, tt.expected, endpoint)
		})
	}
}

func TestBatchMetricsScope(t *testing.T) {
	require.Equal(t, "https://metrics.monitor.azure.com/.default", batchMetricsScope("azurecloud"))
	require.Equal(t, "https://metrics.monitor.azure.cn/.default", batchMetricsScope("azurechinacloud"))
	require.Equal(t, "https://metrics.monitor.azure.us/.default", batchMetricsScope("azuregovernment"))
}

func TestParseResourceTags(t *testing.T) {
	t.Run("map[string]interface{}", func(t *testing.T) {
		tags := map[string]interface{}{
			"env":   "prod",
			"owner": "sre",
			"count": 42, // non-string values should be skipped
		}
		result := parseResourceTags(tags)
		require.Equal(t, map[string]string{"env": "prod", "owner": "sre"}, result)
	})

	t.Run("map[string]string", func(t *testing.T) {
		tags := map[string]string{"env": "prod"}
		result := parseResourceTags(tags)
		require.Equal(t, tags, result)
	})

	t.Run("nil", func(t *testing.T) {
		result := parseResourceTags(nil)
		require.Empty(t, result)
	})
}

func TestReplaceTemplatePlaceholders(t *testing.T) {
	labels := prometheus.Labels{
		"metric":      "cpu_percent",
		"aggregation": "average",
		"unit":        "Percent",
	}

	result := replaceTemplatePlaceholders(
		"azure_{type}_{metric}_{aggregation}_{unit}",
		labels,
		"Microsoft.Compute/virtualMachines",
	)

	require.Equal(t, "azure_Microsoft.Compute/virtualMachines_cpu_percent_average_Percent", result)
	// Fields used in name should be removed from labels (matching upstream behavior)
	require.NotContains(t, labels, "metric")
	require.NotContains(t, labels, "aggregation")
	require.NotContains(t, labels, "unit")
}

func TestProcessBatchResponse(t *testing.T) {
	e := Exporter{
		cfg:    Config{IncludedResourceTags: []string{"owner"}},
		logger: nil, // not used in processBatchResponse
	}

	avgVal := 42.5
	resp := &batchResponse{
		Values: []batchResourceResult{
			{
				ResourceID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
				Value: []batchMetric{
					{
						Name: batchLocalizable{Value: "Percentage CPU"},
						Unit: "Percent",
						Timeseries: []batchTimeseries{
							{
								Data: []batchDataPoint{
									{Average: &avgVal},
								},
							},
						},
					},
				},
			},
		},
	}

	settings := &metrics.RequestMetricSettings{
		ResourceType:   "Microsoft.Compute/virtualMachines",
		Interval:       strPtr("PT1M"),
		Timespan:       "PT5M",
		MetricTemplate: "azure_{type}_{metric}_{aggregation}_{unit}",
		HelpTemplate:   "Azure metric {metric} for {type} with aggregation {aggregation} as {unit}",
	}

	tagsByID := map[string]map[string]string{
		"/subscriptions/sub1/resourcegroups/rg1/providers/microsoft.compute/virtualmachines/vm1": {
			"owner": "sre-team",
		},
	}

	// Create a mock arm client - we'll skip subscription name lookup by passing nil
	// The processBatchResponse will handle the nil case gracefully
	metricList := metrics.NewMetricList()
	e.processBatchResponse(resp, settings, nil, tagsByID, e.cfg, metricList)

	names := metricList.GetMetricNames()
	require.NotEmpty(t, names, "should have produced at least one metric")

	// Check that the metric name follows the template
	found := false
	for _, name := range names {
		if name == "azure_microsoft_compute_virtualmachines_percentage_cpu_average_percent" {
			found = true
			break
		}
	}
	require.True(t, found, "expected metric name matching template, got: %v", names)
}

func TestPublishMetricList(t *testing.T) {
	metricList := metrics.NewMetricList()
	metricList.Add("test_metric", metrics.MetricRow{
		Labels: prometheus.Labels{"foo": "bar"},
		Value:  1.0,
	})
	metricList.SetMetricHelp("test_metric", "A test metric")

	reg := prometheus.NewRegistry()
	publishMetricList(reg, metricList)

	families, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, families, 1)
	require.Equal(t, "test_metric", *families[0].Name)
}

func TestParseISO8601Duration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"PT5M", 5 * time.Minute},
		{"PT1M", 1 * time.Minute},
		{"PT1H", 1 * time.Hour},
		{"PT30M", 30 * time.Minute},
		{"PT1H30M", 90 * time.Minute},
		{"P1D", 24 * time.Hour},
		{"P1DT12H", 36 * time.Hour},
		{"PT90S", 90 * time.Second},
		{"PT1H30M45S", time.Hour + 30*time.Minute + 45*time.Second},
		{"P2D", 48 * time.Hour},
		// Zero/invalid inputs fall back to 5 minutes
		{"", 5 * time.Minute},
		{"garbage", 5 * time.Minute},
		{"PT0S", 5 * time.Minute}, // zero duration falls back
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseISO8601Duration(tt.input)
			require.Equal(t, tt.expected, result, "parseISO8601Duration(%q)", tt.input)
		})
	}
}

func TestComputeTimeWindow(t *testing.T) {
	tests := []struct {
		name     string
		timespan string
		interval *string
		expected time.Duration
	}{
		{
			name:     "timespan wider than interval",
			timespan: "PT5M",
			interval: strPtr("PT1M"),
			expected: 5 * time.Minute,
		},
		{
			name:     "interval wider than timespan — window widens",
			timespan: "PT5M",
			interval: strPtr("PT1H"),
			expected: 1 * time.Hour,
		},
		{
			name:     "nil interval — use timespan as-is",
			timespan: "PT5M",
			interval: nil,
			expected: 5 * time.Minute,
		},
		{
			name:     "empty interval — use timespan as-is",
			timespan: "PT5M",
			interval: strPtr(""),
			expected: 5 * time.Minute,
		},
		{
			name:     "equal interval and timespan",
			timespan: "PT1H",
			interval: strPtr("PT1H"),
			expected: 1 * time.Hour,
		},
		{
			name:     "large interval with small timespan",
			timespan: "PT5M",
			interval: strPtr("P1D"),
			expected: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeTimeWindow(tt.timespan, tt.interval)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessBatchResponse_Dimensions(t *testing.T) {
	e := Exporter{
		cfg:    Config{},
		logger: nil,
	}

	avgVal := 10.0
	resp := &batchResponse{
		Values: []batchResourceResult{
			{
				ResourceID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
				Value: []batchMetric{
					{
						Name: batchLocalizable{Value: "Requests"},
						Unit: "Count",
						Timeseries: []batchTimeseries{
							{
								MetadataValues: []batchMetadataValue{
									{Name: batchLocalizable{Value: "StatusCode"}, Value: "200"},
								},
								Data: []batchDataPoint{{Average: &avgVal}},
							},
						},
					},
				},
			},
		},
	}

	settings := &metrics.RequestMetricSettings{
		ResourceType:   "Microsoft.Compute/virtualMachines",
		Interval:       strPtr("PT1M"),
		Timespan:       "PT5M",
		MetricTemplate: "azure_{type}_{metric}_{aggregation}_{unit}",
		HelpTemplate:   "test help",
	}

	metricList := metrics.NewMetricList()
	e.processBatchResponse(resp, settings, nil, map[string]map[string]string{}, Config{}, metricList)

	// With a single dimension, the label should be "dimension" (not "dimensionStatusCode")
	names := metricList.GetMetricNames()
	require.NotEmpty(t, names)
	for _, name := range names {
		for _, row := range metricList.GetMetricList(name) {
			require.Equal(t, "200", row.Labels["dimension"], "single dimension should use 'dimension' label")
		}
	}
}

func TestProcessBatchResponse_MultiDimensions(t *testing.T) {
	e := Exporter{
		cfg:    Config{},
		logger: nil,
	}

	avgVal := 5.0
	resp := &batchResponse{
		Values: []batchResourceResult{
			{
				ResourceID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
				Value: []batchMetric{
					{
						Name: batchLocalizable{Value: "Requests"},
						Unit: "Count",
						Timeseries: []batchTimeseries{
							{
								MetadataValues: []batchMetadataValue{
									{Name: batchLocalizable{Value: "statusCode"}, Value: "200"},
									{Name: batchLocalizable{Value: "method"}, Value: "GET"},
								},
								Data: []batchDataPoint{{Average: &avgVal}},
							},
						},
					},
				},
			},
		},
	}

	settings := &metrics.RequestMetricSettings{
		ResourceType:   "Microsoft.Compute/virtualMachines",
		Interval:       strPtr("PT1M"),
		Timespan:       "PT5M",
		MetricTemplate: "azure_{type}_{metric}_{aggregation}_{unit}",
		HelpTemplate:   "test help",
	}

	metricList := metrics.NewMetricList()
	e.processBatchResponse(resp, settings, nil, map[string]map[string]string{}, Config{}, metricList)

	names := metricList.GetMetricNames()
	require.NotEmpty(t, names)
	for _, name := range names {
		for _, row := range metricList.GetMetricList(name) {
			require.Equal(t, "200", row.Labels["dimensionStatusCode"], "multi-dimension: statusCode")
			require.Equal(t, "GET", row.Labels["dimensionMethod"], "multi-dimension: method")
		}
	}
}

func TestProcessBatchResponse_AllAggregations(t *testing.T) {
	e := Exporter{
		cfg:    Config{},
		logger: nil,
	}

	total, min, max, avg, count := 100.0, 1.0, 50.0, 25.0, 4.0
	resp := &batchResponse{
		Values: []batchResourceResult{
			{
				ResourceID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
				Value: []batchMetric{
					{
						Name: batchLocalizable{Value: "cpu_percent"},
						Unit: "Percent",
						Timeseries: []batchTimeseries{
							{
								Data: []batchDataPoint{
									{Total: &total, Minimum: &min, Maximum: &max, Average: &avg, Count: &count},
								},
							},
						},
					},
				},
			},
		},
	}

	settings := &metrics.RequestMetricSettings{
		ResourceType:   "Microsoft.Compute/virtualMachines",
		Interval:       strPtr("PT1M"),
		Timespan:       "PT5M",
		MetricTemplate: "azure_{type}_{metric}_{aggregation}_{unit}",
		HelpTemplate:   "test help",
	}

	metricList := metrics.NewMetricList()
	e.processBatchResponse(resp, settings, nil, map[string]map[string]string{}, Config{}, metricList)

	// Should produce 5 metric names (one per aggregation type, since {aggregation}
	// is part of the template and gets baked into the metric name).
	names := metricList.GetMetricNames()
	valueByName := map[string]float64{}
	for _, name := range names {
		for _, row := range metricList.GetMetricList(name) {
			valueByName[name] = row.Value
		}
	}
	require.Len(t, valueByName, 5, "should have 5 metric names for 5 aggregation types, got: %v", names)
	require.Equal(t, 100.0, valueByName["azure_microsoft_compute_virtualmachines_cpu_percent_total_percent"])
	require.Equal(t, 1.0, valueByName["azure_microsoft_compute_virtualmachines_cpu_percent_minimum_percent"])
	require.Equal(t, 50.0, valueByName["azure_microsoft_compute_virtualmachines_cpu_percent_maximum_percent"])
	require.Equal(t, 25.0, valueByName["azure_microsoft_compute_virtualmachines_cpu_percent_average_percent"])
	require.Equal(t, 4.0, valueByName["azure_microsoft_compute_virtualmachines_cpu_percent_count_percent"])
}

func strPtr(s string) *string {
	return &s
}
