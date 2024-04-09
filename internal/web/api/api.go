// Package api implements the HTTP API used for the Grafana Alloy UI.
//
// The API is internal only; it is not stable and shouldn't be relied on
// externally.
package api

import (
	"encoding/json"
	"net/http"
	"path"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/prometheus/prometheus/util/httputil"
)

// AlloyAPI is a wrapper around the component API.
type AlloyAPI struct {
	alloy service.Host
}

// NewAlloyAPI instantiates a new Alloy API.
func NewAlloyAPI(alloy service.Host) *AlloyAPI {
	return &AlloyAPI{alloy: alloy}
}

// RegisterRoutes registers all the API's routes.
func (a *AlloyAPI) RegisterRoutes(urlPrefix string, r *mux.Router) {
	// NOTE(rfratto): {id:.+} is used in routes below to allow the
	// id to contain / characters, which is used by nested module IDs and
	// component IDs.

	r.Handle(path.Join(urlPrefix, "/modules/{moduleID:.+}/components"), httputil.CompressionHandler{Handler: a.listComponentsHandler()})
	r.Handle(path.Join(urlPrefix, "/components"), httputil.CompressionHandler{Handler: a.listComponentsHandler()})
	r.Handle(path.Join(urlPrefix, "/components/{id:.+}"), httputil.CompressionHandler{Handler: a.getComponentHandler()})
	r.Handle(path.Join(urlPrefix, "/peers"), httputil.CompressionHandler{Handler: a.getClusteringPeersHandler()})
}

func (a *AlloyAPI) listComponentsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// moduleID is set from the /modules/{moduleID:.+}/components route above
		// but not from the /components route.
		var moduleID string
		if vars := mux.Vars(r); vars != nil {
			moduleID = vars["moduleID"]
		}

		components, err := a.alloy.ListComponents(moduleID, component.InfoOptions{
			GetHealth: true,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		bb, err := json.Marshal(components)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(bb)
	}
}

func (a *AlloyAPI) getComponentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		requestedComponent := component.ParseID(vars["id"])

		component, err := a.alloy.GetComponent(requestedComponent, component.InfoOptions{
			GetHealth:    true,
			GetArguments: true,
			GetExports:   true,
			GetDebugInfo: true,
		})
		if err != nil {
			http.NotFound(w, r)
			return
		}

		bb, err := json.Marshal(component)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(bb)
	}
}

func (a *AlloyAPI) getClusteringPeersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// TODO(@tpaschalis) Detect if clustering is disabled and propagate to
		// the Typescript code (eg. via the returned status code?).
		svc, found := a.alloy.GetService(cluster.ServiceName)
		if !found {
			http.Error(w, "cluster service not running", http.StatusInternalServerError)
			return
		}
		peers := svc.Data().(cluster.Cluster).Peers()
		bb, err := json.Marshal(peers)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(bb)
	}
}
