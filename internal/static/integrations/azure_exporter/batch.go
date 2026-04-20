package azure_exporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/azure-metrics-exporter/metrics"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/utils/to"
)

const (
	batchAPIVersion       = "2024-02-01"
	maxBatchSize          = 50  // Maximum resources per metrics:getBatch request.
	maxMetricsPerRequest  = 20  // Azure Monitor limit on metric names per request.
	resourceGraphTop      = int32(1000)
)

// batchMetricsEndpoint returns the base URL for the Azure Monitor Batch API for a given region and cloud.
func batchMetricsEndpoint(region, cloudEnv string) string {
	switch strings.ToLower(cloudEnv) {
	case "azurechina", "azurechinacloud":
		return fmt.Sprintf("https://%s.metrics.monitor.azure.cn", region)
	case "usgov", "azuregovernment", "azuregovernmentcloud", "azureusgovernmentcloud":
		return fmt.Sprintf("https://%s.metrics.monitor.azure.us", region)
	default:
		return fmt.Sprintf("https://%s.metrics.monitor.azure.com", region)
	}
}

// batchMetricsScope returns the OAuth2 scope for the Azure Monitor metrics data plane.
func batchMetricsScope(cloudEnv string) string {
	switch strings.ToLower(cloudEnv) {
	case "azurechina", "azurechinacloud":
		return "https://metrics.monitor.azure.cn/.default"
	case "usgov", "azuregovernment", "azuregovernmentcloud", "azureusgovernmentcloud":
		return "https://metrics.monitor.azure.us/.default"
	default:
		return "https://metrics.monitor.azure.com/.default"
	}
}

// discoveredResource holds a resource ID and its Azure region, as returned by Resource Graph.
type discoveredResource struct {
	ID       string
	Location string
	Tags     map[string]string
}

// batchRequest is the JSON body sent to the metrics:getBatch endpoint.
type batchRequest struct {
	ResourceIDs []string `json:"resourceids"`
}

// batchResponse is the top-level JSON response from the metrics:getBatch endpoint.
type batchResponse struct {
	Values []batchResourceResult `json:"values"`
}

// batchResourceResult holds the metrics response for a single resource within a batch.
type batchResourceResult struct {
	StartTime  string        `json:"starttime"`
	EndTime    string        `json:"endtime"`
	ResourceID string        `json:"resourceid"`
	Value      []batchMetric `json:"value"`
}

// batchMetric represents a single metric definition in the batch response.
type batchMetric struct {
	ID         string            `json:"id"`
	Name       batchLocalizable  `json:"name"`
	Unit       string            `json:"unit"`
	Timeseries []batchTimeseries `json:"timeseries"`
}

// batchLocalizable mirrors Azure's localizable string (name + localized value).
type batchLocalizable struct {
	Value          string `json:"value"`
	LocalizedValue string `json:"localizedValue"`
}

// batchTimeseries holds metadata and data points for a timeseries.
type batchTimeseries struct {
	MetadataValues []batchMetadataValue `json:"metadatavalues"`
	Data           []batchDataPoint     `json:"data"`
}

// batchMetadataValue holds a dimension name/value pair.
type batchMetadataValue struct {
	Name  batchLocalizable `json:"name"`
	Value string           `json:"value"`
}

// batchDataPoint holds a single timestamped metric data point.
type batchDataPoint struct {
	TimeStamp time.Time `json:"timeStamp"`
	Total     *float64  `json:"total"`
	Minimum   *float64  `json:"minimum"`
	Maximum   *float64  `json:"maximum"`
	Average   *float64  `json:"average"`
	Count     *float64  `json:"count"`
}

// discoverResourcesWithLocation runs a Resource Graph query that returns resource IDs with their locations.
func discoverResourcesWithLocation(
	ctx context.Context,
	cred azcore.TokenCredential,
	clientOpts *arm.ClientOptions,
	subscriptions []string,
	resourceType string,
	filter string,
) ([]discoveredResource, error) {
	client, err := armresourcegraph.NewClient(cred, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("creating resource graph client: %w", err)
	}

	if filter != "" {
		filter = "| " + filter
	}
	query := strings.TrimSpace(fmt.Sprintf(
		`Resources | where type =~ "%s" %s | project id, location, tags`,
		strings.ReplaceAll(resourceType, "'", "\\'"),
		filter,
	))

	queryFormat := armresourcegraph.ResultFormatObjectArray
	queryTop := resourceGraphTop
	queryRequest := armresourcegraph.QueryRequest{
		Query: to.StringPtr(query),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: &queryFormat,
			Top:          &queryTop,
		},
		Subscriptions: to.SlicePtr(subscriptions),
	}

	result, err := client.Resources(ctx, queryRequest, nil)
	if err != nil {
		return nil, fmt.Errorf("resource graph query: %w", err)
	}

	var resources []discoveredResource
	for {
		resultList, ok := result.Data.([]interface{})
		if !ok || len(resultList) == 0 {
			break
		}

		for _, v := range resultList {
			row, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := row["id"].(string)
			location, _ := row["location"].(string)
			if id == "" || location == "" {
				continue
			}

			tags := parseResourceTags(row["tags"])
			resources = append(resources, discoveredResource{
				ID:       id,
				Location: strings.ToLower(location),
				Tags:     tags,
			})
		}

		if result.SkipToken != nil {
			queryRequest.Options.SkipToken = result.SkipToken
			result, err = client.Resources(ctx, queryRequest, nil)
			if err != nil {
				return nil, fmt.Errorf("resource graph query (paginated): %w", err)
			}
		} else {
			break
		}
	}

	return resources, nil
}

// parseResourceTags converts the tags field from a Resource Graph result into a string map.
func parseResourceTags(tags interface{}) map[string]string {
	result := map[string]string{}
	switch t := tags.(type) {
	case map[string]interface{}:
		for k, v := range t {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
	case map[string]string:
		return t
	}
	return result
}

// resourceGroup is a set of resources sharing the same subscription and region.
type resourceGroup struct {
	SubscriptionID string
	Region         string
	Resources      []discoveredResource
}

// groupResources groups discovered resources by (subscription, region).
func groupResources(resources []discoveredResource) []resourceGroup {
	type key struct {
		sub    string
		region string
	}
	index := map[key]*resourceGroup{}
	var order []key

	for _, r := range resources {
		info, err := azure.ParseResourceID(r.ID)
		if err != nil {
			continue
		}
		k := key{sub: info.SubscriptionID, region: r.Location}
		if _, exists := index[k]; !exists {
			index[k] = &resourceGroup{
				SubscriptionID: info.SubscriptionID,
				Region:         r.Location,
			}
			order = append(order, k)
		}
		index[k].Resources = append(index[k].Resources, r)
	}

	groups := make([]resourceGroup, 0, len(order))
	for _, k := range order {
		groups = append(groups, *index[k])
	}
	return groups
}

// collectBatchMetrics performs the full batch collection flow:
// discover resources, group by (sub, region), call batch API, return Prometheus metrics.
func (e Exporter) collectBatchMetrics(
	ctx context.Context,
	reg *prometheus.Registry,
	cfg Config,
	settings *metrics.RequestMetricSettings,
	azureClient *armclient.ArmClient,
) error {
	resources, err := discoverResourcesWithLocation(
		ctx,
		azureClient.GetCred(),
		azureClient.NewArmClientOptions(),
		cfg.Subscriptions,
		cfg.ResourceType,
		cfg.ResourceGraphQueryFilter,
	)
	if err != nil {
		return fmt.Errorf("batch resource discovery: %w", err)
	}

	if len(resources) == 0 {
		e.logger.Info("no resources discovered, nothing to scrape")
		return nil
	}

	groups := groupResources(resources)
	scope := batchMetricsScope(cfg.AzureCloudEnvironment)

	metricList := metrics.NewMetricList()

	for _, group := range groups {
		endpoint := batchMetricsEndpoint(group.Region, cfg.AzureCloudEnvironment)

		// Batch in chunks of maxBatchSize
		for i := 0; i < len(group.Resources); i += maxBatchSize {
			end := i + maxBatchSize
			if end > len(group.Resources) {
				end = len(group.Resources)
			}
			chunk := group.Resources[i:end]

			ids := make([]string, len(chunk))
			// Build a lookup map for tags by resource ID
			tagsByID := map[string]map[string]string{}
			for j, r := range chunk {
				ids[j] = r.ID
				tagsByID[strings.ToLower(r.ID)] = r.Tags
			}

			// Metrics also need to be batched in chunks of maxMetricsPerRequest (Azure API limit).
			for mi := 0; mi < len(settings.Metrics); mi += maxMetricsPerRequest {
				mEnd := mi + maxMetricsPerRequest
				if mEnd > len(settings.Metrics) {
					mEnd = len(settings.Metrics)
				}
				metricNames := settings.Metrics[mi:mEnd]

				resp, err := e.callBatchAPI(
					ctx, azureClient.GetCred(), scope,
					endpoint, group.SubscriptionID,
					ids, metricNames, settings, cfg,
				)
				if err != nil {
					e.logger.Warnf("batch API call failed for sub=%s region=%s: %v",
						group.SubscriptionID, group.Region, err)
					continue
				}

				e.processBatchResponse(resp, settings, azureClient, tagsByID, cfg, metricList)
			}
		}
	}

	publishMetricList(reg, metricList)
	return nil
}

// callBatchAPI makes a single POST to the metrics:getBatch endpoint.
func (e Exporter) callBatchAPI(
	ctx context.Context,
	cred azcore.TokenCredential,
	scope string,
	endpoint string,
	subscriptionID string,
	resourceIDs []string,
	metricNames []string,
	settings *metrics.RequestMetricSettings,
	cfg Config,
) (*batchResponse, error) {
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return nil, fmt.Errorf("acquiring token: %w", err)
	}

	reqURL := fmt.Sprintf("%s/subscriptions/%s/metrics:getBatch",
		endpoint, subscriptionID)

	// Build query parameters using url.Values for proper encoding.
	params := neturl.Values{}
	params.Set("api-version", batchAPIVersion)
	params.Set("metricnames", strings.Join(metricNames, ","))
	if settings.Interval != nil && *settings.Interval != "" {
		params.Set("interval", *settings.Interval)
	}

	// The batch API (2024-02-01) requires starttime/endtime as ISO 8601 datetimes,
	// not a duration like "PT5M". Compute absolute times, widening the window to
	// at least the interval so that at least one data point is returned.
	now := time.Now().UTC()
	window := computeTimeWindow(settings.Timespan, settings.Interval)
	params.Set("starttime", now.Add(-window).Format(time.RFC3339))
	params.Set("endtime", now.Format(time.RFC3339))

	if len(settings.Aggregations) > 0 {
		params.Set("aggregation", strings.Join(settings.Aggregations, ","))
	}
	if settings.MetricNamespace != "" {
		params.Set("metricnamespace", settings.MetricNamespace)
	} else {
		params.Set("metricnamespace", cfg.ResourceType)
	}
	if settings.MetricFilter != "" {
		params.Set("$filter", settings.MetricFilter)
	}
	if settings.MetricTop != nil {
		params.Set("top", fmt.Sprintf("%d", *settings.MetricTop))
	}
	if settings.MetricOrderBy != "" {
		params.Set("orderby", settings.MetricOrderBy)
	}

	reqURL += "?" + params.Encode()

	body, err := json.Marshal(batchRequest{ResourceIDs: resourceIDs})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var batchResp batchResponse
	if err := json.Unmarshal(respBody, &batchResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return &batchResp, nil
}

// processBatchResponse converts a batch API response into MetricList entries,
// producing the same Prometheus labels as the per-resource path.
func (e Exporter) processBatchResponse(
	resp *batchResponse,
	settings *metrics.RequestMetricSettings,
	azureClient *armclient.ArmClient,
	tagsByID map[string]map[string]string,
	cfg Config,
	metricList *metrics.MetricList,
) {
	for _, resourceResult := range resp.Values {
		resourceID := resourceResult.ResourceID
		azureResource, parseErr := armclient.ParseResourceId(resourceID)
		if parseErr != nil {
			e.logger.Warnf("skipping resource with unparseable ID %q: %v", resourceID, parseErr)
			continue
		}

		subscriptionName := ""
		if azureClient != nil {
			if sub, err := azureClient.GetCachedSubscription(context.Background(), azureResource.Subscription); err == nil && sub != nil {
				subscriptionName = to.String(sub.DisplayName)
			}
		}

		tags := tagsByID[strings.ToLower(resourceID)]

		for _, metric := range resourceResult.Value {
			metricUnit := metric.Unit

			for _, ts := range metric.Timeseries {
				dimensions := map[string]string{}
				for _, md := range ts.MetadataValues {
					dimensions[md.Name.Value] = md.Value
				}

				metricLabels := prometheus.Labels{
					"resourceID":       strings.ToLower(resourceID),
					"subscriptionID":   azureResource.Subscription,
					"subscriptionName": subscriptionName,
					"resourceGroup":    azureResource.ResourceGroup,
					"resourceName":     azureResource.ResourceName,
					"metric":           metric.Name.Value,
					"unit":             metricUnit,
					"interval":         to.String(settings.Interval),
					"timespan":         settings.Timespan,
					"aggregation":      "",
				}

				// Add resource tags as labels
				for _, tagName := range cfg.IncludedResourceTags {
					labelName := "tag_" + tagName
					if val, ok := tags[tagName]; ok {
						metricLabels[labelName] = val
					} else {
						metricLabels[labelName] = ""
					}
				}

				// Handle dimensions (same logic as upstream)
				if len(dimensions) == 1 {
					for _, v := range dimensions {
						metricLabels["dimension"] = v
					}
				} else if len(dimensions) >= 2 {
					for dName, dValue := range dimensions {
						labelName := "dimension" + strings.ToUpper(dName[:1]) + dName[1:]
						metricLabels[labelName] = dValue
					}
				}

				for _, dp := range ts.Data {
					if dp.Total != nil {
						metricLabels["aggregation"] = "total"
						addMetric(metricList, settings, cfg, metricLabels, *dp.Total)
					}
					if dp.Minimum != nil {
						metricLabels["aggregation"] = "minimum"
						addMetric(metricList, settings, cfg, metricLabels, *dp.Minimum)
					}
					if dp.Maximum != nil {
						metricLabels["aggregation"] = "maximum"
						addMetric(metricList, settings, cfg, metricLabels, *dp.Maximum)
					}
					if dp.Average != nil {
						metricLabels["aggregation"] = "average"
						addMetric(metricList, settings, cfg, metricLabels, *dp.Average)
					}
					if dp.Count != nil {
						metricLabels["aggregation"] = "count"
						addMetric(metricList, settings, cfg, metricLabels, *dp.Count)
					}
				}
			}
		}
	}
}

// addMetric builds a metric name from the template and adds it to the MetricList.
func addMetric(
	metricList *metrics.MetricList,
	settings *metrics.RequestMetricSettings,
	cfg Config,
	labels prometheus.Labels,
	value float64,
) {
	// Copy labels to avoid mutation
	metricLabels := prometheus.Labels{}
	for k, v := range labels {
		metricLabels[k] = v
	}

	resourceType := settings.ResourceType
	if settings.MetricNamespace != "" {
		resourceType = settings.MetricNamespace
	}

	// Build metric name from template
	name := settings.MetricTemplate
	if name == "" {
		name = settings.Name
	}

	replacer := strings.NewReplacer(
		"-", "_", " ", "_", "/", "_", ".", "_",
	)

	name = replaceTemplatePlaceholders(name, metricLabels, resourceType)
	name = replacer.Replace(name)
	name = strings.ToLower(name)

	// Build help from template
	help := settings.HelpTemplate
	help = replaceTemplatePlaceholders(help, metricLabels, resourceType)

	metricList.Add(name, metrics.MetricRow{Labels: metricLabels, Value: value})
	metricList.SetMetricHelp(name, help)
}

// replaceTemplatePlaceholders replaces {field} placeholders in a template string.
func replaceTemplatePlaceholders(template string, labels prometheus.Labels, resourceType string) string {
	result := template
	// Simple replacement - find all {field} patterns
	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		fieldName := result[start+1 : end]
		var replacement string
		switch fieldName {
		case "type":
			replacement = resourceType
		default:
			if val, exists := labels[fieldName]; exists {
				replacement = val
				// Remove label from map when used in metric name (matches upstream behavior)
				delete(labels, fieldName)
			}
		}
		result = result[:start] + replacement + result[end+1:]
	}
	return result
}

// computeTimeWindow returns the time window to use for a batch API request.
// It ensures the window is at least as wide as the interval so that at least
// one data point can be returned. For example, if interval is PT1H but timespan
// is only PT5M, the window is widened to PT1H.
func computeTimeWindow(timespan string, interval *string) time.Duration {
	window := parseISO8601Duration(timespan)
	if interval != nil && *interval != "" {
		intervalDur := parseISO8601Duration(*interval)
		if intervalDur > window {
			window = intervalDur
		}
	}
	return window
}

// parseISO8601Duration parses a simple ISO 8601 duration string (e.g., "PT5M", "PT1H", "P1D")
// into a Go time.Duration. Supports days (D), hours (H), minutes (M), and seconds (S).
// Does not support year (Y), month (M before T), or week (W) designators.
func parseISO8601Duration(s string) time.Duration {
	s = strings.TrimPrefix(s, "P")
	var d time.Duration
	inTime := false
	numBuf := ""
	for _, c := range s {
		switch {
		case c == 'T':
			inTime = true
		case c >= '0' && c <= '9' || c == '.':
			numBuf += string(c)
		default:
			val := 0.0
			if numBuf != "" {
				fmt.Sscanf(numBuf, "%f", &val)
				numBuf = ""
			}
			switch {
			case c == 'D' && !inTime:
				d += time.Duration(val * float64(24*time.Hour))
			case c == 'H' && inTime:
				d += time.Duration(val * float64(time.Hour))
			case c == 'M' && inTime:
				d += time.Duration(val * float64(time.Minute))
			case c == 'S' && inTime:
				d += time.Duration(val * float64(time.Second))
			}
		}
	}
	// Default to 5 minutes if parsing produces zero
	if d == 0 {
		d = 5 * time.Minute
	}
	return d
}

// publishMetricList registers all collected metrics with the Prometheus registry.
func publishMetricList(reg *prometheus.Registry, metricList *metrics.MetricList) {
	for _, metricName := range metricList.GetMetricNames() {
		gauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: metricList.GetMetricHelp(metricName),
			},
			metricList.GetMetricLabelNames(metricName),
		)
		reg.MustRegister(gauge)

		for _, row := range metricList.GetMetricList(metricName) {
			gauge.With(row.Labels).Set(row.Value)
		}
	}
}
