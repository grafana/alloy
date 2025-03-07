package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
)

// prometheusTargetSearchDebugInfo handles searches for Prometheus targets' debug info across all peers in the cluster
func prometheusTargetSearchDebugInfo(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to get query
		var requestBody struct {
			SearchQuery string `json:"searchQuery"`
		}

		// Decode JSON body
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Use search query from request body
		query := requestBody.SearchQuery

		// Search for targets
		response, err := searchPrometheusTargetsDebugInfos(query, host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Marshal the result to JSON
		result, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Write the response
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(result)
	}
}

// prometheusTargetDebugInfo handles requests for debug information about Prometheus targets
func prometheusTargetDebugInfo(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Find all prometheus.scrape components and extract their debug info
		response, err := getPrometheusTargetsDebugInfo(host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Marshal the result to JSON with indentation for pretty formatting
		result, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(result)
	}
}

func getPrometheusTargetsDebugInfo(host service.Host) (PrometheusTargetDebugResponse, error) {
	// Initialize response
	response := PrometheusTargetDebugResponse{
		Components: make(map[string]ComponentDebugInfo),
	}

	// Get all prometheus target components
	prometheusComponents, err := listPrometheusTargetsComponents(host)
	if err != nil {
		return response, fmt.Errorf("failed to list Prometheus components: %w", err)
	}

	// For each component, get detailed debug info
	for _, comp := range prometheusComponents {
		compId := comp.ID.String()
		// Get component with debug info
		info, err := host.GetComponent(comp.ID, component.InfoOptions{
			GetDebugInfo: true,
		})
		if err != nil {
			errMsg := fmt.Sprintf("failed to get info for component %s: %v", compId, err)
			response.Errors = append(response.Errors, errMsg)
			continue
		}

		scrapeStatus, ok := info.DebugInfo.(scrape.ScraperStatus)
		if !ok {
			errMsg := fmt.Sprintf("component %s does not have expected scrape debug info", compId)
			response.Errors = append(response.Errors, errMsg)
			continue
		}

		response.Components[compId] = ComponentDebugInfo{
			TargetsStatus: make([]TargetStatus, len(scrapeStatus.TargetStatus)),
		}

		for i, target := range scrapeStatus.TargetStatus {
			response.Components[compId].TargetsStatus[i] = TargetStatus{
				JobName:            target.JobName,
				URL:                target.URL,
				Health:             target.Health,
				Labels:             target.Labels,
				LastError:          target.LastError,
				LastScrape:         target.LastScrape.Format(time.RFC3339),
				LastScrapeDuration: fmt.Sprintf("%v", target.LastScrapeDuration),
			}
		}
	}

	return response, nil
}

func searchPrometheusTargetsDebugInfos(query string, host service.Host) (SearchPrometheusTargetsResponse, error) {
	// Initialize results with empty maps to avoid null values in JSON
	response := SearchPrometheusTargetsResponse{
		Results: make(map[string]PrometheusTargetDebugResponse),
	}

	// Determine protocol based on TLS configuration
	tlsEnabled, err := isTLSEnabled(host)
	if err != nil {
		return response, fmt.Errorf("error checking TLS status: %v", err)
	}
	protocol := "http"
	if tlsEnabled {
		protocol = "https"
	}

	// For all peers, get the details
	clusterSvc, found := host.GetService(cluster.ServiceName)
	if !found {
		return response, fmt.Errorf("cluster service not running")
	}
	peers := clusterSvc.Data().(cluster.Cluster).Peers()

	// TODO: this could be done concurrently for all peers to speed things up
	// TODO: we could send the query term to all peers as well so the responses come filtered already
	for _, p := range peers {
		// Construct the URL to get prometheus targets debug info from the peer
		peerURL := fmt.Sprintf("%s://%s/api/v0/web/tools/prometheus-targets-debug-info", protocol, p.Addr)

		// Create a new request
		peerReq, err := http.NewRequest("GET", peerURL, nil)
		if err != nil {
			errMsg := fmt.Sprintf("Error creating request for peer %s: %v", p.Name, err)
			response.Errors = append(response.Errors, errMsg)
			continue
		}

		// Make the request to the peer
		resp, err := defaultHTTPClient.Do(peerReq)
		if err != nil {
			errMsg := fmt.Sprintf("Error requesting debug info from peer %s: %v", p.Name, err)
			response.Errors = append(response.Errors, errMsg)
			continue
		}

		// Process the response
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				errMsg := fmt.Sprintf("Error reading response from peer %s: %v", p.Name, err)
				response.Errors = append(response.Errors, errMsg)
				continue
			}

			var debugInfo PrometheusTargetDebugResponse
			if err := json.Unmarshal(body, &debugInfo); err != nil {
				errMsg := fmt.Sprintf("Error parsing response from peer %s: %v", p.Name, err)
				response.Errors = append(response.Errors, errMsg)
				continue
			}

			// If query is provided, filter the targets
			if query != "" {
				filteredDebugInfo := filterTargetsBasedOnQuery(debugInfo, query)
				response.Results[p.Name] = filteredDebugInfo
			} else {
				response.Results[p.Name] = debugInfo
			}
		} else {
			errMsg := fmt.Sprintf("Error response from peer %s: %d", p.Name, resp.StatusCode)
			response.Errors = append(response.Errors, errMsg)
			continue
		}
	}

	return response, nil
}

// filterTargetsBasedOnQuery filters targets that match the query string in URL or labels
func filterTargetsBasedOnQuery(debugInfo PrometheusTargetDebugResponse, query string) PrometheusTargetDebugResponse {
	result := PrometheusTargetDebugResponse{
		Components: make(map[string]ComponentDebugInfo),
		Errors:     debugInfo.Errors,
	}

	for compID, compInfo := range debugInfo.Components {
		var filteredTargets []TargetStatus

		for _, target := range compInfo.TargetsStatus {
			// Check if query matches URL
			if matchString(target.URL, query) {
				filteredTargets = append(filteredTargets, target)
				continue
			}

			// Check if query matches any label key or value
			for key, value := range target.Labels {
				if matchString(key, query) || matchString(value, query) {
					filteredTargets = append(filteredTargets, target)
					break // break the iteration over labels
				}
			}
		}

		result.Components[compID] = ComponentDebugInfo{
			TargetsStatus: filteredTargets,
		}
	}

	return result
}

// matchString tries to match a string with query, first using plaintext search,
// then falling back to regex if needed
func matchString(s, query string) bool {
	// First try simple case-insensitive string containment
	if strings.Contains(strings.ToLower(s), strings.ToLower(query)) {
		return true
	}

	// Fall back to regex match if plaintext doesn't find a match
	matched, err := regexp.MatchString(query, s)
	if err == nil && matched {
		return true
	}

	return false
}

// listPrometheusTargetsComponents returns components that represent Prometheus targets
func listPrometheusTargetsComponents(host service.Host) ([]*component.Info, error) {
	allComponents, err := host.ListComponents("", component.InfoOptions{})
	if err != nil {
		return nil, err
	}

	// Target component types that have Prometheus targets DebugInfo
	supportedComponents := []string{
		"prometheus.scrape",
	}

	var prometheusComponents []*component.Info
	for _, comp := range allComponents {
		for _, supported := range supportedComponents {
			if comp.ComponentName == supported {
				prometheusComponents = append(prometheusComponents, comp)
				break // Once we've matched, we can stop checking other types
			}
		}
	}

	return prometheusComponents, nil
}

type SearchPrometheusTargetsResponse struct {
	Results map[string]PrometheusTargetDebugResponse `json:"results"`
	Errors  []string                                 `json:"errors"`
}

type PrometheusTargetDebugResponse struct {
	Components map[string]ComponentDebugInfo `json:"components"`
	Errors     []string                      `json:"errors"`
}

type ComponentDebugInfo struct {
	TargetsStatus []TargetStatus `json:"targetsStatus"`
}

type TargetStatus struct {
	JobName            string            `json:"jobName"`
	URL                string            `json:"url"`
	Health             string            `json:"health"`
	Labels             map[string]string `json:"labels"`
	LastError          string            `json:"lastError"`
	LastScrape         string            `json:"lastScrape"`
	LastScrapeDuration string            `json:"lastScrapeDuration"`
}
