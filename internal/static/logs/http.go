package logs

import (
	"net/http"
	"sort"

	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/static/metrics/cluster/configapi"
	"github.com/grafana/loki/clients/pkg/promtail/targets/target"
	"github.com/prometheus/common/model"
)

// WireAPI adds API routes to the provided mux router.
func (l *Logs) WireAPI(r *mux.Router) {
	r.HandleFunc("/agent/api/v1/logs/instances", l.ListInstancesHandler).Methods("GET")
	r.HandleFunc("/agent/api/v1/logs/targets", l.ListTargetsHandler).Methods("GET")
}

// ListInstancesHandler writes the set of currently running instances to the http.ResponseWriter.
func (l *Logs) ListInstancesHandler(w http.ResponseWriter, _ *http.Request) {
	instances := l.instances
	instanceNames := make([]string, 0, len(instances))
	for instance := range instances {
		instanceNames = append(instanceNames, instance)
	}
	sort.Strings(instanceNames)

	err := configapi.WriteResponse(w, http.StatusOK, instanceNames)
	if err != nil {
		level.Error(l.l).Log("msg", "failed to write response", "err", err)
	}
}

// ListTargetsHandler retrieves the full set of targets across all instances and shows
// information on them.
func (l *Logs) ListTargetsHandler(w http.ResponseWriter, r *http.Request) {
	instances := l.instances
	allTargets := make(map[string]TargetSet, len(instances))
	for instName, inst := range instances {
		allTargets[instName] = inst.promtail.ActiveTargets()
	}
	listTargetsHandler(allTargets).ServeHTTP(w, r)
}

func listTargetsHandler(targets map[string]TargetSet) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		resp := ListTargetsResponse{}
		for instance, tset := range targets {
			for key, targets := range tset {
				for _, tgt := range targets {
					resp = append(resp, TargetInfo{
						InstanceName:     instance,
						TargetGroup:      key,
						Type:             tgt.Type(),
						DiscoveredLabels: tgt.DiscoveredLabels(),
						Labels:           tgt.Labels(),
						Ready:            tgt.Ready(),
						Details:          tgt.Details(),
					})
				}
			}
		}
		_ = configapi.WriteResponse(rw, http.StatusOK, resp)
	})
}

// TargetSet is a set of targets for an individual scraper.
type TargetSet map[string][]target.Target

// ListTargetsResponse is returned by the ListTargetsHandler.
type ListTargetsResponse []TargetInfo

// TargetInfo describes a specific target.
type TargetInfo struct {
	InstanceName string `json:"instance"`
	TargetGroup  string `json:"target_group"`

	Type             target.TargetType `json:"type"`
	Labels           model.LabelSet    `json:"labels"`
	DiscoveredLabels model.LabelSet    `json:"discovered_labels"`
	Ready            bool              `json:"ready"`
	Details          interface{}       `json:"details"`
}
