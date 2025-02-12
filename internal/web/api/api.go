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
	"time"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/remotecfg"
	"github.com/prometheus/prometheus/util/httputil"
)

// AlloyAPI is a wrapper around the component API.
type AlloyAPI struct {
	alloy           service.Host
	CallbackManager livedebugging.CallbackManager
	logger          log.Logger
}

// NewAlloyAPI instantiates a new Alloy API.
func NewAlloyAPI(alloy service.Host, CallbackManager livedebugging.CallbackManager, l log.Logger) *AlloyAPI {
	return &AlloyAPI{alloy: alloy, CallbackManager: CallbackManager, logger: l}
}

// RegisterRoutes registers all the API's routes.
func (a *AlloyAPI) RegisterRoutes(urlPrefix string, r *mux.Router) {
	// NOTE(rfratto): {id:.+} is used in routes below to allow the
	// id to contain / characters, which is used by nested module IDs and
	// component IDs.

	r.Handle(path.Join(urlPrefix, "/modules/{moduleID:.+}/components"), httputil.CompressionHandler{Handler: listComponentsHandler(a.alloy)})
	r.Handle(path.Join(urlPrefix, "/remotecfg/modules/{moduleID:.+}/components"), httputil.CompressionHandler{Handler: listComponentsHandlerRemoteCfg(a.alloy)})

	r.Handle(path.Join(urlPrefix, "/components"), httputil.CompressionHandler{Handler: listComponentsHandler(a.alloy)})
	r.Handle(path.Join(urlPrefix, "/remotecfg/components"), httputil.CompressionHandler{Handler: listComponentsHandlerRemoteCfg(a.alloy)})

	r.Handle(path.Join(urlPrefix, "/components/{id:.+}"), httputil.CompressionHandler{Handler: getComponentHandler(a.alloy)})
	r.Handle(path.Join(urlPrefix, "/remotecfg/components/{id:.+}"), httputil.CompressionHandler{Handler: getComponentHandlerRemoteCfg(a.alloy)})

	r.Handle(path.Join(urlPrefix, "/peers"), httputil.CompressionHandler{Handler: getClusteringPeersHandler(a.alloy)})
	r.Handle(path.Join(urlPrefix, "/debug/{id:.+}"), liveDebugging(a.alloy, a.CallbackManager, a.logger))

	r.Handle(path.Join(urlPrefix, "/graph"), graph(a.alloy, a.CallbackManager, a.logger))
	r.Handle(path.Join(urlPrefix, "/graph/{moduleID:.+}"), graph(a.alloy, a.CallbackManager, a.logger))
}

func listComponentsHandler(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listComponentsHandlerInternal(host, w, r)
	}
}

func listComponentsHandlerRemoteCfg(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, found := host.GetService(remotecfg.ServiceName)
		if !found {
			http.Error(w, "remote config service not available", http.StatusInternalServerError)
			return
		}

		data := svc.Data().(remotecfg.Data)
		if data.Host == nil {
			http.Error(w, "remote config service startup in progress", http.StatusInternalServerError)
			return
		}
		listComponentsHandlerInternal(data.Host, w, r)
	}
}

func listComponentsHandlerInternal(host service.Host, w http.ResponseWriter, r *http.Request) {
	// moduleID is set from the /modules/{moduleID:.+}/components route above
	// but not from the /components route.
	var moduleID string
	if vars := mux.Vars(r); vars != nil {
		moduleID = vars["moduleID"]
	}

	components, err := host.ListComponents(moduleID, component.InfoOptions{
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

func getComponentHandler(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		getComponentHandlerInternal(host, w, r)
	}
}

func getComponentHandlerRemoteCfg(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, found := host.GetService(remotecfg.ServiceName)
		if !found {
			http.Error(w, "remote config service not available", http.StatusInternalServerError)
			return
		}

		data := svc.Data().(remotecfg.Data)
		if data.Host == nil {
			http.Error(w, "remote config service startup in progress", http.StatusInternalServerError)
			return
		}

		getComponentHandlerInternal(data.Host, w, r)
	}
}

func getComponentHandlerInternal(host service.Host, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestedComponent := component.ParseID(vars["id"])

	component, err := host.GetComponent(requestedComponent, component.InfoOptions{
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

func getClusteringPeersHandler(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// TODO(@tpaschalis) Detect if clustering is disabled and propagate to
		// the Typescript code (eg. via the returned status code?).
		svc, found := host.GetService(cluster.ServiceName)
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

type dataKey struct {
	ComponentID livedebugging.ComponentID
	Type        livedebugging.DataType
}

func graph(_ service.Host, callbackManager livedebugging.CallbackManager, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var moduleID livedebugging.ModuleID
		if vars := mux.Vars(r); vars != nil {
			moduleID = livedebugging.ModuleID(vars["moduleID"])
		}

		window := setWindow(w, r.URL.Query().Get("window"))

		dataCh := make(chan *livedebugging.Data, 1000)
		dataMap := make(map[dataKey]liveDebuggingData)

		ctx := r.Context()
		id := livedebugging.CallbackID(uuid.New().String())

		droppedData := false
		err := callbackManager.AddCallbackMulti(id, moduleID, func(data *livedebugging.Data) {
			select {
			case <-ctx.Done():
				return
			default:
				select {
				case dataCh <- data:
				default:
					if !droppedData {
						level.Warn(logger).Log("msg", "data throughput is very high, not all debugging data can be sent to the graph")
						droppedData = true
					}
				}
			}
		})

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer func() {
			close(dataCh)
			callbackManager.DeleteCallbackMulti(id, moduleID)
		}()

		ticker := time.NewTicker(time.Duration(window) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case data := <-dataCh:
				// Aggregate incoming data
				key := dataKey{ComponentID: data.ComponentID, Type: data.Type}
				if existing, exists := dataMap[key]; exists {
					existing.Count += data.Count
				} else {
					// The data is ignored for the graph.
					dataMap[key] = liveDebuggingData{
						ComponentID:        string(data.ComponentID),
						Count:              data.Count,
						Type:               string(data.Type),
						TargetComponentIDs: data.TargetComponentIDs,
					}
				}

			case <-ticker.C:
				// Flush aggregated data
				var builder strings.Builder
				for _, data := range dataMap {
					data.Rate = float64(data.Count) / float64(window)
					jsonData, err := json.Marshal(data)
					if err != nil {
						continue
					}
					builder.Write(jsonData)
					builder.WriteString("|;|")
				}

				// Add an empty limiter to show the lack of data
				if builder.Len() == 0 {
					builder.WriteString("|;|")
				}

				_, writeErr := w.Write([]byte(builder.String()))
				if writeErr != nil {
					return
				}
				w.(http.Flusher).Flush()

				for k := range dataMap {
					delete(dataMap, k)
				}

			case <-ctx.Done():
				return
			}
		}
	}
}

func liveDebugging(_ service.Host, callbackManager livedebugging.CallbackManager, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		componentID := livedebugging.ComponentID(vars["id"])

		dataCh := make(chan string, 1000)
		ctx := r.Context()

		sampleProb := setSampleProb(w, r.URL.Query().Get("sampleProb"))

		id := livedebugging.CallbackID(uuid.New().String())

		droppedData := false
		err := callbackManager.AddCallback(id, componentID, func(data *livedebugging.Data) {
			select {
			case <-ctx.Done():
				return
			default:
				if sampleProb < 1 && rand.Float64() > sampleProb {
					return
				}
				// Avoid blocking the channel when the channel is full
				select {
				case dataCh <- data.DataFunc():
				default:
					if !droppedData {
						level.Warn(logger).Log("msg", "data throughput is very high, not all debugging data can be sent the live debugging stream")
						droppedData = true
					}
				}
			}
		})

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		flushTicker := time.NewTicker(time.Second)

		defer func() {
			close(dataCh)
			callbackManager.DeleteCallback(id, componentID)
			flushTicker.Stop()
		}()

		for {
			select {
			case data := <-dataCh:
				var builder strings.Builder
				builder.WriteString(string(data))
				// |;| delimiter is added at the end of every chunk
				builder.WriteString("|;|")
				_, writeErr := w.Write([]byte(builder.String()))
				if writeErr != nil {
					return
				}
			case <-flushTicker.C:
				w.(http.Flusher).Flush()
			case <-ctx.Done():
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

// window is expected to be in seconds, between 1 and 60.
func setWindow(w http.ResponseWriter, windowParam string) (window int64) {
	window = 5
	if windowParam != "" {
		var err error
		window, err = strconv.ParseInt(windowParam, 10, 64)
		if err != nil || window < 1 || window > 60 {
			http.Error(w, "Invalid window", http.StatusBadRequest)
			return 5
		}
	}
	return window
}
