// Package api implements the HTTP API used for the Grafana Alloy UI.
//
// The API is internal only; it is not stable and shouldn't be relied on
// externally.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/grafana/ckit/peer"
	"github.com/prometheus/prometheus/util/httputil"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/remotecfg"
)

// AlloyAPI is a wrapper around the component API.
type AlloyAPI struct {
	alloy           service.Host
	CallbackManager livedebugging.CallbackManager
}

// NewAlloyAPI instantiates a new Alloy API.
func NewAlloyAPI(alloy service.Host, CallbackManager livedebugging.CallbackManager) *AlloyAPI {
	return &AlloyAPI{alloy: alloy, CallbackManager: CallbackManager}
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
	r.Handle(path.Join(urlPrefix, "/peers/{peerName}/components/{id:.+}"), httputil.CompressionHandler{Handler: getPeerComponentHandler(a.alloy)})

	r.Handle(path.Join(urlPrefix, "/debug/{id:.+}"), liveDebugging(a.alloy, a.CallbackManager))

	r.Handle(path.Join(urlPrefix, "/tools/prometheus-target-search"), httputil.CompressionHandler{Handler: prometheusTargetSearchHandler()})
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

	comp, err := host.GetComponent(requestedComponent, component.InfoOptions{
		GetHealth:    true,
		GetArguments: true,
		GetExports:   true,
		GetDebugInfo: true,
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}

	bb, err := json.Marshal(comp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(bb)
}

func getClusteringPeersHandler(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		fmt.Println("======= getClusteringPeersHandler")
		// TODO(@tpaschalis) Detect if clustering is disabled and propagate to
		// the Typescript code (eg. via the returned status code?).
		svc, found := host.GetService(cluster.ServiceName)
		if !found {
			http.Error(w, "cluster service not running", http.StatusNotFound)
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

func liveDebugging(_ service.Host, callbackManager livedebugging.CallbackManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		componentID := livedebugging.ComponentID(vars["id"])

		dataCh := make(chan string, 1000)
		ctx := r.Context()

		sampleProb := setSampleProb(w, r.URL.Query().Get("sampleProb"))

		id := livedebugging.CallbackID(uuid.New().String())

		err := callbackManager.AddCallback(id, componentID, func(data string) {
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

// getPeerComponentHandler creates a handler to fetch component details from a specific peer in a cluster
func getPeerComponentHandler(host service.Host) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("======= getPeerComponentHandler")
		vars := mux.Vars(r)
		peerName := vars["peerName"]
		componentID := vars["id"]

		// find the target peer from cluster service
		clusterSvc, found := host.GetService(cluster.ServiceName)
		if !found {
			http.Error(w, "cluster service not running", http.StatusNotFound)
			return
		}
		peers := clusterSvc.Data().(cluster.Cluster).Peers()
		var targetPeer *peer.Peer
		for _, p := range peers {
			if p.Name == peerName {
				targetPeer = &p
				break
			}
		}
		if targetPeer == nil {
			http.Error(w, fmt.Sprintf("peer '%s' not found", peerName), http.StatusNotFound)
			return
		}

		// Get the HTTP service to check if TLS is enabled
		httpSvc, found := host.GetService(httpservice.ServiceName)
		if !found {
			http.Error(w, "HTTP service not running", http.StatusInternalServerError)
			return
		}

		// Determine protocol based on TLS configuration
		protocol := "http"
		if httpService, ok := httpSvc.(*httpservice.Service); ok && httpService.IsTLS() {
			protocol = "https"
		}

		// Construct the URL to forward the request to the peer
		peerURL := fmt.Sprintf("%s://%s/api/v0/web/components/%s", protocol, targetPeer.Addr, componentID)

		fmt.Println("======= peerURL", peerURL)

		// Create a new request to forward to the peer
		peerReq, err := http.NewRequestWithContext(r.Context(), "GET", peerURL, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("error creating request to peer: %v", err), http.StatusInternalServerError)
			return
		}

		// Forward relevant headers
		for k, v := range r.Header {
			if k == "Accept-Encoding" || k == "Accept" || k == "Authorization" {
				peerReq.Header[k] = v
			}
		}

		// Perform the request to the peer
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(peerReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("error requesting data from peer '%s': %v", peerName, err), http.StatusBadGateway)
			return
		}
		defer func(body io.ReadCloser) { _ = body.Close() }(resp.Body)

		fmt.Println("======= resp.StatusCode", resp.StatusCode)
		fmt.Println("======= resp.Header", resp.Header)

		// Copy the status code
		w.WriteHeader(resp.StatusCode)

		// Copy the headers
		for k, v := range resp.Header {
			for _, val := range v {
				w.Header().Add(k, val)
			}
		}

		// Copy the body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("error reading response body: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Println("======= resp.Body", string(body))
		_, _ = w.Write(body)
	}
}

// prometheusTargetSearchHandler creates a handler to search for Prometheus targets
func prometheusTargetSearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body
		var requestData struct {
			SearchQuery string `json:"searchQuery"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Create a placeholder response
		response := searchPrometheusTargets(requestData.SearchQuery)

		// Set content type header
		w.Header().Set("Content-Type", "application/json")

		// Marshal and write the response
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to generate response", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(jsonResponse)
	}
}

func searchPrometheusTargets(query string) Targets {
	return Targets{
		Targets: []Target{
			{
				Instance:    "test-01",
				ComponentID: "prometheus.scrape",
				Labels:      map[string]string{"instance": "foo", "team": "bar", "query": query},
				DebugInfo:   map[string]string{"status": "up", "lastScrape": "yesterday"},
			},
		},
	}
}

type Target struct {
	Instance    string            `json:"instance"`
	ComponentID string            `json:"componentID"`
	Labels      map[string]string `json:"labels"`
	DebugInfo   map[string]string `json:"debugInfo"`
}

type Targets struct {
	Targets []Target `json:"targets"`
}
