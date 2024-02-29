package metrics

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/static/metrics/cluster/configapi"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
)

// WireAPI adds API routes to the provided mux router.
func (a *Agent) WireAPI(r *mux.Router) {
	a.cluster.WireAPI(r)

	// Backwards compatible endpoints. Use endpoints with `metrics` prefix instead
	r.HandleFunc("/agent/api/v1/instances", a.ListInstancesHandler).Methods("GET")
	r.HandleFunc("/agent/api/v1/targets", a.ListTargetsHandler).Methods("GET")

	r.HandleFunc("/agent/api/v1/metrics/instances", a.ListInstancesHandler).Methods("GET")
	r.HandleFunc("/agent/api/v1/metrics/targets", a.ListTargetsHandler).Methods("GET")
	r.HandleFunc("/agent/api/v1/metrics/instance/{instance}/write", a.PushMetricsHandler).Methods("POST")
}

// ListInstancesHandler writes the set of currently running instances to the http.ResponseWriter.
func (a *Agent) ListInstancesHandler(w http.ResponseWriter, _ *http.Request) {
	cfgs := a.mm.ListConfigs()
	instanceNames := make([]string, 0, len(cfgs))
	for k := range cfgs {
		instanceNames = append(instanceNames, k)
	}
	sort.Strings(instanceNames)

	err := configapi.WriteResponse(w, http.StatusOK, instanceNames)
	if err != nil {
		level.Error(a.logger).Log("msg", "failed to write response", "err", err)
	}
}

// ListTargetsHandler retrieves the full set of targets across all instances and shows
// information on them.
func (a *Agent) ListTargetsHandler(w http.ResponseWriter, r *http.Request) {
	instances := a.mm.ListInstances()
	allTagets := make(map[string]TargetSet, len(instances))
	for instName, inst := range instances {
		allTagets[instName] = inst.TargetsActive()
	}
	ListTargetsHandler(allTagets).ServeHTTP(w, r)
}

// ListTargetsHandler renders a mapping of instance to target set.
func ListTargetsHandler(targets map[string]TargetSet) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		resp := ListTargetsResponse{}

		for instance, tset := range targets {
			for key, targets := range tset {
				for _, tgt := range targets {
					var lastError string
					if scrapeError := tgt.LastError(); scrapeError != nil {
						lastError = scrapeError.Error()
					}

					resp = append(resp, TargetInfo{
						InstanceName: instance,
						TargetGroup:  key,

						Endpoint:         tgt.URL().String(),
						State:            string(tgt.Health()),
						DiscoveredLabels: tgt.DiscoveredLabels(),
						Labels:           tgt.Labels(),
						LastScrape:       tgt.LastScrape(),
						ScrapeDuration:   tgt.LastScrapeDuration().Milliseconds(),
						ScrapeError:      lastError,
					})
				}
			}
		}

		sort.Slice(resp, func(i, j int) bool {
			// sort by instance, then target group, then job label, then instance label
			var (
				iInstance      = resp[i].InstanceName
				iTargetGroup   = resp[i].TargetGroup
				iJobLabel      = resp[i].Labels.Get(model.JobLabel)
				iInstanceLabel = resp[i].Labels.Get(model.InstanceLabel)

				jInstance      = resp[j].InstanceName
				jTargetGroup   = resp[j].TargetGroup
				jJobLabel      = resp[j].Labels.Get(model.JobLabel)
				jInstanceLabel = resp[j].Labels.Get(model.InstanceLabel)
			)

			switch {
			case iInstance != jInstance:
				return iInstance < jInstance
			case iTargetGroup != jTargetGroup:
				return iTargetGroup < jTargetGroup
			case iJobLabel != jJobLabel:
				return iJobLabel < jJobLabel
			default:
				return iInstanceLabel < jInstanceLabel
			}
		})

		_ = configapi.WriteResponse(rw, http.StatusOK, resp)
	})
}

// TargetSet is a set of targets for an individual scraper.
type TargetSet map[string][]*scrape.Target

// ListTargetsResponse is returned by the ListTargetsHandler.
type ListTargetsResponse []TargetInfo

// TargetInfo describes a specific target.
type TargetInfo struct {
	InstanceName string `json:"instance"`
	TargetGroup  string `json:"target_group"`

	Endpoint         string        `json:"endpoint"`
	State            string        `json:"state"`
	Labels           labels.Labels `json:"labels"`
	DiscoveredLabels labels.Labels `json:"discovered_labels"`
	LastScrape       time.Time     `json:"last_scrape"`
	ScrapeDuration   int64         `json:"scrape_duration_ms"`
	ScrapeError      string        `json:"scrape_error"`
}

// PushMetricsHandler provides a way to POST data directly into
// an instance's WAL.
func (a *Agent) PushMetricsHandler(w http.ResponseWriter, r *http.Request) {
	// Get instance name.
	instanceName, err := getInstanceName(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get the metrics instance and serve the request.
	managedInstance, err := a.InstanceManager().GetInstance(instanceName)
	if err != nil || managedInstance == nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	managedInstance.WriteHandler().ServeHTTP(w, r)
}

// getInstanceName uses gorilla/mux's route variables to extract the
// "instance" variable. If not found, getInstanceName will return an error.
func getInstanceName(r *http.Request) (string, error) {
	vars := mux.Vars(r)
	name := vars["instance"]
	name, err := url.PathUnescape(name)
	if err != nil {
		return "", fmt.Errorf("could not decode instance name: %w", err)
	}
	return name, nil
}
