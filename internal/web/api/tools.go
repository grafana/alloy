package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
)

// prometheusTargetSearchHandler handles searches for Prometheus targets
func prometheusTargetSearchHandler(host service.Host) http.HandlerFunc {
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
		response, err := searchPrometheusTargets(query, host)
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

// searchPrometheusTargets searches for Prometheus targets across all peers
func searchPrometheusTargets(query string, host service.Host) (SearchPrometheusTargetsResponse, error) {
	fmt.Println("======= searchPrometheusTargets", query)

	// Initialize results with empty maps to avoid null values in JSON
	response := SearchPrometheusTargetsResponse{
		Results: make(map[string]InstanceResults),
		Errors:  []string{},
	}

	// Get Prometheus target components
	prometheusComponents, err := listPrometheusTargetsComponents(host)
	if err != nil {
		return response, err
	}
	fmt.Println("======= prometheus components:")
	for _, comp := range prometheusComponents {
		fmt.Println(comp.ID.String())
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

	// For all peers, get the component details
	clusterSvc, found := host.GetService(cluster.ServiceName)
	if !found {
		return response, fmt.Errorf("cluster service not running")
	}
	peers := clusterSvc.Data().(cluster.Cluster).Peers()

	for _, p := range peers {
		fmt.Println("======= processing peer", p.Name)

		// Initialize instance data for this peer
		instanceData := InstanceResults{
			Components: make(map[string]TargetResults),
		}
		foundMatches := false

		// For each Prometheus component, make a request to the peer
		for _, comp := range prometheusComponents {
			// Construct the URL to get component details from the peer
			peerURL := fmt.Sprintf("%s://%s/api/v0/web/components/%s", protocol, p.Addr, comp.ID)
			fmt.Println("======= requesting", peerURL)

			// Create a new request
			peerReq, err := http.NewRequest("GET", peerURL, nil)
			if err != nil {
				errMsg := fmt.Sprintf("Error creating request for peer %s, component %s: %v", p.Name, comp.ID, err)
				fmt.Println(errMsg)
				response.Errors = append(response.Errors, errMsg)
				continue
			}

			// Make the request to the peer
			resp, err := defaultHTTPClient.Do(peerReq)
			if err != nil {
				errMsg := fmt.Sprintf("Error requesting data from peer %s, component %s: %v", p.Name, comp.ID, err)
				fmt.Println(errMsg)
				response.Errors = append(response.Errors, errMsg)
				continue
			}

			// Process the response
			if resp.StatusCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					errMsg := fmt.Sprintf("Error reading response from peer %s, component %s: %v", p.Name, comp.ID, err)
					fmt.Println(errMsg)
					response.Errors = append(response.Errors, errMsg)
					continue
				}

				var compInfo map[string]interface{}
				if err := json.Unmarshal(body, &compInfo); err != nil {
					errMsg := fmt.Sprintf("Error parsing response from peer %s, component %s: %v", p.Name, comp.ID, err)
					fmt.Println(errMsg)
					response.Errors = append(response.Errors, errMsg)
					continue
				}

				// Search for targets in the component
				matchingArgs := searchTargetsInSection(query, compInfo, "arguments")
				matchingExports := searchTargetsInSection(query, compInfo, "exports")

				// Only add to results if we found matches
				if len(matchingArgs) > 0 || len(matchingExports) > 0 {
					// Get component ID
					moduleID, _ := compInfo["moduleID"].(string)
					localID, _ := compInfo["localID"].(string)
					componentID := moduleID + "/" + localID

					// Add target info to the instance data
					instanceData.Components[componentID] = TargetResults{
						Args:    matchingArgs,
						Exports: matchingExports,
					}
					foundMatches = true
				}
			} else {
				errMsg := fmt.Sprintf("Error response from peer %s, component %s: %d", p.Name, comp.ID, resp.StatusCode)
				fmt.Println(errMsg)
				response.Errors = append(response.Errors, errMsg)
				continue
			}
		}

		// Only add instance to results if we found matches in any component
		if foundMatches {
			response.Results[p.Name] = instanceData
		}
	}

	return response, nil
}

// listPrometheusTargetsComponents returns components that represent Prometheus targets
func listPrometheusTargetsComponents(host service.Host) ([]*component.Info, error) {
	allComponents, err := host.ListComponents("", component.InfoOptions{})
	if err != nil {
		return nil, err
	}

	// Target component types that might have Prometheus targets
	targetTypes := []string{
		"discovery.",
		"prometheus.",
		"remote.prometheus.",
	}

	var prometheusComponents []*component.Info
	for _, comp := range allComponents {
		for _, targetType := range targetTypes {
			if strings.HasPrefix(comp.ComponentName, targetType) {
				prometheusComponents = append(prometheusComponents, comp)
				break // Once we've matched, we can stop checking other types
			}
		}
	}

	return prometheusComponents, nil
}

// searchTargetsInSection looks for targets in arguments or exports that match the query
func searchTargetsInSection(query string, compInfo map[string]interface{}, section string) []map[string]string {
	var results []map[string]string

	// Check if the section exists
	sectionData, ok := compInfo[section].([]interface{})
	if !ok {
		return results
	}

	// Look for targets in the section
	for _, arg := range sectionData {
		argMap, ok := arg.(map[string]interface{})
		if !ok {
			continue
		}

		// Get the value field
		valueObj, ok := argMap["value"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check if the value is an array
		valueType, ok := valueObj["type"].(string)
		if !ok || valueType != "array" {
			continue
		}

		// Grab the array
		targetsArray, ok := valueObj["value"].([]interface{})
		if !ok {
			continue
		}

		// Process each target object
		for _, targetObj := range targetsArray {
			target, ok := targetObj.(map[string]interface{})
			if !ok {
				continue
			}

			// Check if it has a value array (which contains key-value pairs)
			targetValue, ok := target["value"].([]interface{})
			if !ok {
				continue
			}

			// Convert the target object to a simple map[string]string
			targetMap := make(map[string]string)
			matchFound := false

			// Process each key-value pair in the target
			for _, kv := range targetValue {
				kvMap, ok := kv.(map[string]interface{})
				if !ok {
					continue
				}

				key, ok := kvMap["key"].(string)
				if !ok {
					continue
				}

				valueObj, ok := kvMap["value"].(map[string]interface{})
				if !ok {
					continue
				}

				valueStr, ok := valueObj["value"].(string)
				if !ok {
					continue
				}

				// Add to our map
				targetMap[key] = valueStr

				// Check if this value matches our query - either exact match or regex
				if strings.Contains(valueStr, query) || strings.Contains(key, query) {
					matchFound = true
				} else {
					// Try as regex
					matched, err := regexp.MatchString(query, key)
					if err == nil && matched {
						matchFound = true
					}
					matched, err = regexp.MatchString(query, valueStr)
					if err == nil && matched {
						matchFound = true
					}
				}
			}

			// If we found a match, add this target to our results
			if matchFound {
				results = append(results, targetMap)
			}
		}
	}

	return results
}

type SearchPrometheusTargetsResponse struct {
	Results map[string]InstanceResults `json:"results"`
	Errors  []string                   `json:"errors"`
}

type InstanceResults struct {
	Components map[string]TargetResults `json:"components"`
}

type TargetResults struct {
	Args    []map[string]string `json:"args"`
	Exports []map[string]string `json:"exports"`
}
