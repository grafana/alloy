package receive_http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"

	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
)

func (c *Component) getDebugInfoEndpoints() []debuginfo.Endpoint {
	c.mut.Lock()
	defer c.mut.Unlock()
	var endpoints []debuginfo.Endpoint
	for _, appendable := range c.appendables {
		endpoints = append(endpoints, appendable.DebugInfoEndpoints()...)
	}
	return endpoints
}

func (c *Component) firstEndpoint() (*debuginfo.Endpoint, error) {
	endpoints := c.getDebugInfoEndpoints()
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no downstream endpoints available")
	}
	return &endpoints[0], nil
}

func (c *Component) ShouldInitiateUpload(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
	ep, err := c.firstEndpoint()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return ep.ConnectClient.ShouldInitiateUpload(ctx, req)
}

func (c *Component) UploadFinished(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (*connect.Response[debuginfov1alpha1.UploadFinishedResponse], error) {
	ep, err := c.firstEndpoint()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return ep.ConnectClient.UploadFinished(ctx, req)
}

func (c *Component) UploadHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ep, err := c.firstEndpoint()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		gnuBuildID := mux.Vars(r)["gnu_build_id"]
		uploadURL := strings.TrimRight(ep.BaseURL, "/") + "/debuginfo.v1alpha1.DebuginfoService/Upload/" + gnuBuildID

		proxyReq, err := http.NewRequestWithContext(r.Context(), "POST", uploadURL, r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("create proxy request: %v", err), http.StatusInternalServerError)
			return
		}
		for k, vs := range r.Header {
			for _, v := range vs {
				proxyReq.Header.Add(k, v)
			}
		}

		resp, err := ep.HTTPClient.Do(proxyReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("downstream upload: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)

		w.WriteHeader(resp.StatusCode)
	})
}
