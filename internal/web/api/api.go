// Package api implements the HTTP API used for the Grafana Alloy UI.
//
// The API is internal only; it is not stable and shouldn't be relied on
// externally.
package api

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/prometheus/prometheus/util/httputil"
)

// AlloyAPI is a wrapper around the component API.
type AlloyAPI struct {
	alloy                  service.Host
	debuggingStreamHandler livedebugging.DebugStreamManager
}

// NewAlloyAPI instantiates a new Alloy API.
func NewAlloyAPI(alloy service.Host, debuggingStreamHandler livedebugging.DebugStreamManager) *AlloyAPI {
	return &AlloyAPI{alloy: alloy, debuggingStreamHandler: debuggingStreamHandler}
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
	r.Handle(path.Join(urlPrefix, "/debug/{id:.+}"), a.startDebugStream())
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

func (a *AlloyAPI) startDebugStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		componentID := vars["id"]

		// Buffer of 1000 entries to handle load spikes and prevent this functionality from eating up too much memory.
		dataCh := make(chan string, 1000)
		ctx := r.Context()

		sampleProb := setSampleProb(w, r.URL.Query().Get("sampleProb"))

		a.debuggingStreamHandler.SetStream(componentID, func(data string) {
			select {
			case <-ctx.Done():
				return
			default:
				if sampleProb < 1 && rand.Float64() > sampleProb {
					return
				}
				// Avoid blocking the channel when the channel is full
				select {
				case dataCh <- data:
				default:
				}
			}
		})

		stopStreaming := func() {
			close(dataCh)
			a.debuggingStreamHandler.DeleteStream(componentID)
		}

		for {
			select {
			case data := <-dataCh:
				var builder strings.Builder
				builder.WriteString(data)
				// |;| delimiter is added at the end of every chunk
				builder.WriteString("|;|")
				_, writeErr := w.Write([]byte(builder.String()))
				if writeErr != nil {
					stopStreaming()
					return
				}
				// TODO: flushing at a regular interval might be better performance wise
				w.(http.Flusher).Flush()
			case <-ctx.Done():
				stopStreaming()
				return
			}
		}
	}
}

func setSampleProb(w http.ResponseWriter, sampleProbParam string) (sampleProb float64) {
	sampleProb = 1.0
	if sampleProbParam != "" {
		var err error
		sampleProb, err = strconv.ParseFloat(sampleProbParam, 64)
		if err != nil || sampleProb < 0 || sampleProb > 1 {
			http.Error(w, "Invalid sample probability", http.StatusBadRequest)
			return 1.0
		}
	}
	return sampleProb
}
