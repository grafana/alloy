package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	httpservice "github.com/grafana/alloy/internal/service/http"
)

// getClusterTargetDebugInfo handles searches for Prometheus targets' debug info across all peers in the cluster
func getClusterTargetDebugInfo(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		// Search for targets
		response, err := searchClusterTargetsDebugInfo(query, host)
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

// getInstanceTargetDebugInfo handles requests for debug information about Prometheus targets
func getInstanceTargetDebugInfo(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		// Find all prometheus.scrape components and extract their debug info
		response, err := getLocalTargetsDebugInfo(host, query)
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

func getLocalTargetsDebugInfo(host service.Host, query string) (PrometheusTargetDebugResponse, error) {
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

		componentInfo := ComponentDebugInfo{
			TargetsStatus: []TargetStatus{},
		}

		for _, target := range scrapeStatus.TargetStatus {
			if targetStatus := filterAndBuildTargetStatus(target, query); targetStatus != nil {
				componentInfo.TargetsStatus = append(componentInfo.TargetsStatus, *targetStatus)
			}
		}

		response.Components[compId] = componentInfo
	}

	return response, nil
}

// filterAndBuildTargetStatus checks if a target matches the query and if so,
// builds and returns a TargetStatus. Returns nil if target doesn't match the query.
func filterAndBuildTargetStatus(target scrape.TargetStatus, query string) *TargetStatus {
	if shouldIncludeTarget(target, query) {
		return &TargetStatus{
			JobName:            target.JobName,
			URL:                target.URL,
			Health:             target.Health,
			Labels:             target.Labels,
			LastError:          target.LastError,
			LastScrape:         target.LastScrape.Format(time.RFC3339),
			LastScrapeDuration: fmt.Sprintf("%v", target.LastScrapeDuration),
		}
	}

	return nil
}

// shouldIncludeTarget determines if a target should be included based on the query
func shouldIncludeTarget(target scrape.TargetStatus, query string) bool {
	if query == "" {
		return true
	}

	if matchString(target.URL, query) {
		return true
	}

	for key, value := range target.Labels {
		if matchString(key, query) || matchString(value, query) {
			return true
		}
	}

	return false
}

func searchClusterTargetsDebugInfo(query string, host service.Host) (SearchPrometheusTargetsResponse, error) {
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
	for _, p := range peers {
		peerURL := fmt.Sprintf("%s://%s/api/v0/web/tools/instance-prom-targets-debug-info", protocol, p.Addr)
		if query != "" {
			peerURL = fmt.Sprintf("%s?query=%s", peerURL, url.QueryEscape(query))
		}

		peerReq, err := http.NewRequest("GET", peerURL, nil)
		if err != nil {
			errMsg := fmt.Sprintf("Error creating request for peer %s: %v", p.Name, err)
			response.Errors = append(response.Errors, errMsg)
			continue
		}

		resp, err := defaultHTTPClient.Do(peerReq)
		if err != nil {
			errMsg := fmt.Sprintf("Error requesting debug info from peer %s: %v", p.Name, err)
			response.Errors = append(response.Errors, errMsg)
			continue
		}

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

			response.Results[p.Name] = debugInfo
		} else {
			errMsg := fmt.Sprintf("Error response from peer %s: %d", p.Name, resp.StatusCode)
			response.Errors = append(response.Errors, errMsg)
			continue
		}
	}

	return response, nil
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

// isTLSEnabled checks if TLS is enabled for the HTTP service
func isTLSEnabled(host service.Host) (bool, error) {
	httpSvc, found := host.GetService(httpservice.ServiceName)
	if !found {
		return false, fmt.Errorf("HTTP service not running")
	}

	httpService, ok := httpSvc.(*httpservice.Service)
	if !ok {
		return false, fmt.Errorf("HTTP service has unexpected type")
	}

	return httpService.IsTLS(), nil
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
