package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/remotecfg"
)

// TargetsResponse represents the API response for targets endpoint.
type TargetsResponse struct {
	Status string       `json:"status"`
	Data   []TargetData `json:"data"`
}

// TargetData represents information about a single scrape target.
type TargetData struct {
	ComponentID        string            `json:"component_id"`
	JobName            string            `json:"job"`
	URL                string            `json:"url"`
	Health             string            `json:"health"`
	Labels             map[string]string `json:"labels"`
	DiscoveredLabels   map[string]string `json:"discovered_labels,omitempty"`
	LastScrape         string            `json:"last_scrape"`
	LastScrapeDuration string            `json:"last_scrape_duration,omitempty"`
	LastError          string            `json:"last_error,omitempty"`
}

func getTargetsHandler(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		getTargetsHandlerInternal(host, w, r)
	}
}

func getTargetsHandlerRemoteCfg(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remoteCfgHost, err := remotecfg.GetHost(host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		getTargetsHandlerInternal(remoteCfgHost, w, r)
	}
}

func getTargetsHandlerInternal(host service.Host, w http.ResponseWriter, r *http.Request) {
	var moduleID string
	if vars := mux.Vars(r); vars != nil {
		moduleID = vars["moduleID"]
	}

	// Optional query parameters for filtering
	jobFilter := r.URL.Query().Get("job")
	healthFilter := r.URL.Query().Get("health")
	componentFilter := r.URL.Query().Get("component")

	components, err := host.ListComponents(moduleID, component.InfoOptions{
		GetHealth: false,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var allTargets []TargetData

	for _, info := range components {
		if info.Component == nil {
			continue
		}

		// Filter by component ID if specified
		componentID := info.ID.String()
		if componentFilter != "" && componentID != componentFilter {
			continue
		}

		// Check if component implements TargetsProvider
		provider, ok := info.Component.(component.TargetsProvider)
		if !ok {
			continue
		}

		targets := provider.GetTargets()
		for _, t := range targets {
			// Apply filters
			if jobFilter != "" && t.JobName != jobFilter {
				continue
			}
			if healthFilter != "" && t.State != healthFilter {
				continue
			}

			var lastScrape string
			if !t.LastScrape.IsZero() {
				lastScrape = t.LastScrape.Format(time.RFC3339)
			}

			var lastScrapeDuration string
			if t.LastScrapeDuration > 0 {
				lastScrapeDuration = t.LastScrapeDuration.String()
			}

			allTargets = append(allTargets, TargetData{
				ComponentID:        componentID,
				JobName:            t.JobName,
				URL:                t.Endpoint,
				Health:             t.State,
				Labels:             t.Labels,
				DiscoveredLabels:   t.DiscoveredLabels,
				LastScrape:         lastScrape,
				LastScrapeDuration: lastScrapeDuration,
				LastError:          t.LastError,
			})
		}
	}

	response := TargetsResponse{
		Status: "success",
		Data:   allTargets,
	}

	bb, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(bb)
}
